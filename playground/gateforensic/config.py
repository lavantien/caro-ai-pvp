import pathlib

MODULE_DIR = pathlib.Path(__file__).resolve().parent
REPO_ROOT = MODULE_DIR.parents[1]
ARENA_OUT = REPO_ROOT / "playground" / "arena" / "out"
OUT_DIR = MODULE_DIR / "out"
SQL_DIR = MODULE_DIR / "sql"
RUN_LOG = OUT_DIR / "gateforensic-run.log"
REPORT_MD = ARENA_OUT / "gateforensic-report.md"
INTEGRITY_TXT = OUT_DIR / "integrity.txt"

GEOMETRIES = ("hex", "square")
TIERS = ("easy", "medium", "hard")
MASTER = "master"
TC_INDEXES = (1, 2)

ARM_RESOLVED = "resolved"
ARM_PROVISIONAL = "provisional"
ARM_ATTEMPT = "attempt"

ARMS = [
    {"name": "hex-passgates-hard-tc1", "geometry": "hex", "opp": "hard", "tc": 1, "role": ARM_RESOLVED},
    {"name": "hex-passgates-hard-tc2", "geometry": "hex", "opp": "hard", "tc": 2, "role": ARM_RESOLVED},
    {"name": "hex-passgates-medium-tc1", "geometry": "hex", "opp": "medium", "tc": 1, "role": ARM_RESOLVED},
    {"name": "hex-passgates-medium-tc2", "geometry": "hex", "opp": "medium", "tc": 2, "role": ARM_RESOLVED},
    {"name": "hex-passgates-easy-tc1", "geometry": "hex", "opp": "easy", "tc": 1, "role": ARM_RESOLVED},
    {"name": "hex-passgates-easy-tc1.attempt1", "geometry": "hex", "opp": "easy", "tc": 1, "role": ARM_ATTEMPT},
    {"name": "hex-passgates-easy-tc2", "geometry": "hex", "opp": "easy", "tc": 2, "role": ARM_RESOLVED},
    {"name": "square-passgates-hard-tc1", "geometry": "square", "opp": "hard", "tc": 1, "role": ARM_RESOLVED},
    {"name": "square-passgates-hard-tc2", "geometry": "square", "opp": "hard", "tc": 2, "role": ARM_RESOLVED},
    {"name": "square-passgates-medium-tc1", "geometry": "square", "opp": "medium", "tc": 1, "role": ARM_RESOLVED},
    {"name": "square-passgates-medium-tc2", "geometry": "square", "opp": "medium", "tc": 2, "role": ARM_RESOLVED},
    {"name": "square-passgates-easy-tc1", "geometry": "square", "opp": "easy", "tc": 1, "role": ARM_RESOLVED},
    {"name": "square-passgates-easy-tc2", "geometry": "square", "opp": "easy", "tc": 2, "role": ARM_PROVISIONAL},
]

THRESHOLDS = {"easy": 0.90, "medium": 0.75, "hard": 0.60}
WALD_Z = 1.96

MATE_BASE = 1 << 30
MATE_STEP = 16

TAGS = ("BOOK", "PONDER", "VCT", "VCF")
PONDER_VERDICTS = ("adopt", "reject", "miss")
WINNER_TOKENS = ("red", "blue", "draw")
RESULT_WIN = "W"
RESULT_LOSS = "L"
RESULT_DRAW = "D"
RESULT_UNFINISHED = "U"

PHASE_EARLY_MAX = 20
PHASE_MID_MAX = 40
PHASE_NAMES = ("early", "mid", "late")

MOVE_COLUMNS = [
    "arm", "geometry", "opp", "tc_idx", "source", "provisional", "attempt",
    "game", "block", "master_seat", "result", "game_moves", "move_no", "color",
    "is_master", "cell", "depth", "nodes", "nps", "ebf", "tt_pct", "hf_pct",
    "fh1_pct", "score_raw", "score_cp", "mate_dist", "thr", "t", "alloc",
    "tag", "pv_len",
]
GAME_COLUMNS = [
    "arm", "geometry", "opp", "tc_idx", "source", "provisional", "attempt",
    "game", "block", "master_seat", "result", "winner", "moves",
]
EVENT_COLUMNS = ["arm", "source", "provisional", "lineno", "kind", "game", "detail"]
VERDICT_COLUMNS = [
    "arm", "geometry", "opp", "tc_idx", "source", "provisional", "attempt",
    "status", "opp_field", "tc_field", "wd", "threshold", "n", "lb", "ub",
    "cap", "lineno",
]
EXTEND_COLUMNS = ["arm", "source", "provisional", "lineno", "block", "max_blocks", "n", "wd", "lb", "ub"]
LEDGER_COLUMNS = [
    "arm", "source", "role", "provisional", "lines_total", "moves", "games",
    "events", "extends", "verdicts", "blanks", "bad", "truncated", "sha256",
]
BADLINE_COLUMNS = ["arm", "source", "lineno", "kind", "text"]

PARITY_TAGGED_EXCLUDE = TAGS
