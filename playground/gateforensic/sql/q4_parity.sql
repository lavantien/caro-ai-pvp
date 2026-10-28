SELECT
    arm,
    geometry,
    opp,
    tc_idx,
    is_master,
    count(*) AS search_moves,
    round(avg(depth), 2) AS depth_mean,
    median(depth) AS depth_median,
    round(avg(nodes) / 1e6, 2) AS nodes_m_mean,
    round(avg(nps) / 1e6, 2) AS nps_m_mean,
    round(avg(tt_pct), 1) AS tt_mean,
    round(avg(hf_pct), 1) AS hf_mean,
    round(avg(fh1_pct), 1) AS fh1_mean,
    round(avg(t), 2) AS t_mean,
    round(avg(alloc), 2) AS alloc_mean,
    round(avg(CASE WHEN alloc > 0 THEN t / alloc END), 3) AS t_over_alloc
FROM moves
WHERE coalesce(tag, '') = ''
GROUP BY arm, geometry, opp, tc_idx, is_master
ORDER BY geometry, opp, tc_idx, is_master, arm
