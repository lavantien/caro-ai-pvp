import pathlib
import random
import shutil
import tempfile
import unittest
import unittest.mock

import analyze
import config
import ingest
import parse

MLINE_FULL = "g1 M2, Blue, H6, d=10, n=19.07m, nps=4.24m, ebf=5.3, tt=9%, hf=58%, fh1=91%, s=+300, thr=4, t=4.50, alloc=4.50, pv=H6 H5 I6"
MLINE_TAGGED = "g9 M130, Blue, E6, d=1, n=2.98m, nps=3.03m, ebf=32.0, tt=35%, hf=11%, fh1=96%, s=-7600, thr=4, t=0.99, alloc=2.00, [BOOK], pv=E6"
MLINE_MATE = "g1 M88, Red, C11, d=11, n=42.42m, nps=6.3m, ebf=5.8, tt=16%, hf=33%, fh1=93%, s=M7, thr=6, t=6.73, alloc=6.76, pv=C11 D12"
MLINE_NEGMATE = "g1 M89, Blue, D12, d=10, n=22.13m, nps=3.66m, ebf=5.4, tt=12%, hf=59%, fh1=91%, s=-M10, thr=4, t=6.04, alloc=6.10, pv=D12"
MLINE_ZERO = "g1 M1, Red, H8, d=0, n=0, nps=0, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=+0, thr=6, t=0.00, alloc=4.50, [BOOK], pv=H8"
GAME_START = "game 21: master (red) vs hard (blue)"
GAME_RESULT = "game 20: hard (red) vs master, winner draw over 256 moves"
PONDER = "g1 ponder hard m4 reject"
EXTEND = "extend: block 1/3 n=20 wd=0.350 lb=0.141 ub=0.559"
VERDICT_DONE = "hex-passgates: DONE"
VERDICT_FAILED = "hex-passgates: FAILED opp=medium tc=2 wd=0.350 threshold=0.750 n=20 lb=0.141 ub=0.559 cap=false"
VERDICT_SHORT = "hex-passgates: FAILED opp=easy tc=1 wd=0.850 threshold=0.900"
ARM_START = "2026-10-28T05:27:53 arm square-passgates-easy-tc2 start: bin\\arena-freeze.exe -geometry square -passgate easy -tc 2 -games 20 -gateMaxBlocks 3"
ARM_EXIT = "2026-10-28T05:27:53 arm square-passgates-easy-tc2 exit 0"
ARM_ADOPTED = "2026-10-28T05:27:53 arm square-passgates-easy-tc2 adopted"
DESCRIPTOR = "passgate master vs easy tc 3+2 games 20 blocks 3 geometry square"
DESCRIPTOR_NOBLOCKS = "passgate master vs hard tc 2+1 games 20 geometry hex"


class ParseTest(unittest.TestCase):
    def test_mline_full(self):
        m = parse.parse_mline(MLINE_FULL)
        self.assertEqual(m["game"], 1)
        self.assertEqual(m["move_no"], 2)
        self.assertEqual(m["color"], "Blue")
        self.assertEqual(m["cell"], "H6")
        self.assertEqual(m["depth"], 10)
        self.assertEqual(m["nodes"], 19_070_000)
        self.assertEqual(m["nps"], 4_240_000)
        self.assertEqual(m["score_cp"], 300)
        self.assertIsNone(m["mate_dist"])
        self.assertEqual(m["tag"], "")
        self.assertEqual(m["pv_len"], 3)

    def test_mline_tagged(self):
        m = parse.parse_mline(MLINE_TAGGED)
        self.assertEqual(m["tag"], "BOOK")
        self.assertEqual(m["score_cp"], -7600)
        self.assertEqual(m["pv_len"], 1)

    def test_mline_mate_scores(self):
        m = parse.parse_mline(MLINE_MATE)
        self.assertEqual(m["score_cp"], config.MATE_BASE - 7 * config.MATE_STEP)
        self.assertEqual(m["mate_dist"], 7)
        m = parse.parse_mline(MLINE_NEGMATE)
        self.assertEqual(m["score_cp"], -(config.MATE_BASE - 10 * config.MATE_STEP))
        self.assertEqual(m["mate_dist"], -10)

    def test_mline_zero(self):
        m = parse.parse_mline(MLINE_ZERO)
        self.assertEqual(m["score_cp"], 0)
        self.assertEqual(m["nodes"], 0)
        self.assertEqual(m["pv_len"], 1)

    def test_mline_rejects(self):
        bad = [
            MLINE_FULL.replace("d=10", "d=x"),
            MLINE_FULL.replace(", thr=4", ""),
            MLINE_FULL + " tail",
            MLINE_FULL.replace("s=+300", "s=abc"),
            MLINE_FULL.replace("Blue", "Green"),
        ]
        for line in bad:
            self.assertIsNone(parse.parse_mline(line), line)

    def test_mline_empty_pv_accepted(self):
        m = parse.parse_mline(MLINE_FULL.replace("pv=H6 H5 I6", "pv="))
        self.assertEqual(m["pv_len"], 0)

    def test_game_lines(self):
        g = parse.parse_game_start(GAME_START)
        self.assertEqual((g["game"], g["red"], g["blue"]), (21, "master", "hard"))
        r = parse.parse_game_result(GAME_RESULT)
        self.assertEqual((r["game"], r["winner"], r["moves"]), (20, "draw", 256))

    def test_ponder_extend_verdict(self):
        p = parse.parse_ponder(PONDER)
        self.assertEqual((p["game"], p["side"], p["move_no"], p["verdict"]), (1, "hard", 4, "reject"))
        e = parse.parse_extend_block(EXTEND)
        self.assertEqual((e["block"], e["max_blocks"], e["n"], e["wd"]), (1, 3, 20, 0.350))
        v = parse.parse_verdict(VERDICT_DONE)
        self.assertEqual(v["status"], "DONE")
        v = parse.parse_verdict(VERDICT_FAILED)
        self.assertEqual(v["status"], "FAILED")
        self.assertEqual(v["opp_field"], "medium")
        self.assertEqual(v["n"], 20)
        self.assertEqual(v["cap"], "false")
        v = parse.parse_verdict(VERDICT_SHORT)
        self.assertEqual((v["status"], v["wd"], v["n"]), ("FAILED", 0.850, None))

    def test_arm_and_descriptor(self):
        self.assertEqual(parse.parse_arm_start(ARM_START)["arm"], "square-passgates-easy-tc2")
        self.assertEqual(parse.parse_arm_exit(ARM_EXIT)["exit"], 0)
        self.assertIsNotNone(parse.parse_arm_adopted(ARM_ADOPTED))
        d = parse.parse_descriptor(DESCRIPTOR)
        self.assertEqual((d["opp"], d["initial_min"], d["inc_sec"], d["games"], d["blocks"]), ("easy", 3, 2, 20, 3))
        d = parse.parse_descriptor(DESCRIPTOR_NOBLOCKS)
        self.assertIsNone(d["blocks"])

    def test_classify(self):
        cases = [
            (MLINE_FULL, parse.KIND_MLINE),
            (GAME_START, parse.KIND_GAME_START),
            (GAME_RESULT, parse.KIND_GAME_RESULT),
            (PONDER, parse.KIND_PONDER),
            (EXTEND, parse.KIND_EXTEND_BLOCK),
            ("extend: needed, run another block", parse.KIND_EXTEND_NOTE),
            (VERDICT_FAILED, parse.KIND_VERDICT),
            (ARM_START, parse.KIND_ARM_START),
            (ARM_EXIT, parse.KIND_ARM_EXIT),
            (ARM_ADOPTED, parse.KIND_ARM_ADOPTED),
            (DESCRIPTOR, parse.KIND_DESCRIPTOR),
            ("", parse.KIND_BLANK),
            ("garbage line", None),
        ]
        for line, kind in cases:
            self.assertEqual(parse.classify(line), kind, line)

    def test_parse_compact(self):
        self.assertEqual(parse.parse_compact("0"), 0)
        self.assertEqual(parse.parse_compact("20.2k"), 20_200)
        self.assertEqual(parse.parse_compact("3m"), 3_000_000)
        self.assertEqual(parse.parse_compact("51.38k"), 51_380)


class FuzzTest(unittest.TestCase):
    SEEDS = 20261028
    CORPUS = [
        MLINE_FULL, MLINE_TAGGED, MLINE_MATE, MLINE_ZERO, GAME_START, GAME_RESULT,
        PONDER, EXTEND, VERDICT_FAILED, VERDICT_SHORT, ARM_START, DESCRIPTOR,
    ]

    def test_mutations_never_raise(self):
        rng = random.Random(self.SEEDS)
        mutations = 0
        for base in self.CORPUS:
            for _ in range(400):
                line = list(base)
                for _ in range(rng.randint(1, 4)):
                    i = rng.randrange(len(line))
                    choice = rng.random()
                    if choice < 0.4:
                        line[i] = rng.choice("abcXYZ0123456789 ,.%-[]")
                    elif choice < 0.7:
                        line[i] = ""
                    else:
                        line.insert(i, rng.choice(",.=[]% "))
                candidate = "".join(line)
                for fn in (
                    parse.parse_mline, parse.parse_game_start, parse.parse_game_result,
                    parse.parse_ponder, parse.parse_extend_block, parse.parse_verdict,
                    parse.parse_descriptor, parse.parse_arm_start, parse.parse_arm_exit,
                ):
                    fn(candidate)
                    mutations += 1
                parse.classify(candidate)
        self.assertGreater(mutations, 10_000)

    def test_truncations_never_raise(self):
        for base in self.CORPUS:
            for cut in range(len(base)):
                candidate = base[:cut]
                parse.parse_mline(candidate)
                parse.parse_verdict(candidate)
                parse.classify(candidate)


class PipelineTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.mkdtemp(prefix="gateforensic-test-")
        cls.out = pathlib.Path(cls.tmp)
        patches = [
            unittest.mock.patch.object(config, "OUT_DIR", cls.out),
            unittest.mock.patch.object(config, "RUN_LOG", cls.out / "run.log"),
            unittest.mock.patch.object(config, "REPORT_MD", cls.out / "report.md"),
            unittest.mock.patch.object(config, "INTEGRITY_TXT", cls.out / "integrity.txt"),
        ]
        for p in patches:
            p.start()
            cls.addClassCleanup(p.stop)
        ingest.run(allow_provisional=True)
        analyze.run()

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp, ignore_errors=True)

    def test_ledger_covers_all_arms(self):
        rows = (self.out / "ledger.csv").read_text(encoding="utf-8").splitlines()
        self.assertEqual(len(rows), 1 + len(config.ARMS))

    def test_no_bad_lines(self):
        body = (self.out / "bad-lines.csv").read_text(encoding="utf-8").strip()
        self.assertEqual(body.splitlines(), ["arm,source,lineno,kind,text"])

    def test_expected_game_totals(self):
        import csv

        with open(self.out / "games.csv", encoding="utf-8", newline="") as f:
            rows = list(csv.DictReader(f))
        per_arm = {}
        for r in rows:
            per_arm[r["arm"]] = per_arm.get(r["arm"], 0) + 1
        self.assertEqual(per_arm["hex-passgates-easy-tc1"], 60)
        self.assertEqual(per_arm["square-passgates-hard-tc1"], 60)
        self.assertEqual(per_arm["square-passgates-medium-tc1"], 40)
        self.assertEqual(per_arm["square-passgates-medium-tc2"], 20)

    def test_provisional_arm_marked(self):
        import csv

        with open(self.out / "ledger.csv", encoding="utf-8", newline="") as f:
            rows = {r["arm"]: r for r in csv.DictReader(f)}
        for arm, row in rows.items():
            self.assertEqual(row["provisional"], "false", arm)
        self.assertIn("square-passgates-easy-tc2", rows)

    def test_integrity_all_pass(self):
        text = self.out / "integrity.txt"
        self.assertTrue(text.exists())
        for line in text.read_text(encoding="utf-8").splitlines():
            self.assertFalse(line.startswith("FAIL"), line)

    def test_report_and_parquet_emitted(self):
        self.assertTrue((self.out / "report.md").exists())
        self.assertTrue((self.out / "moves.parquet").exists())
        self.assertTrue((self.out / "events.parquet").exists())


if __name__ == "__main__":
    unittest.main(verbosity=2)
