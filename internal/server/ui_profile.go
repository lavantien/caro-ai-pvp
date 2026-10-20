package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type wldRecord struct {
	Wins   int
	Losses int
	Draws  int
}

func (r *wldRecord) add(won, draw bool) {
	switch {
	case draw:
		r.Draws++
	case won:
		r.Wins++
	default:
		r.Losses++
	}
}

type TierRecord struct {
	Tier string
	wldRecord
}

type ProfileGame struct {
	GameID          int64
	PlayedAt        int64
	Opponent        string
	OpponentProfile string
	Result          string
	Turns           int
}

type PlayerProfile struct {
	User   User
	Games  int
	All    wldRecord
	PvP    wldRecord
	Bot    wldRecord
	Tiers  []TierRecord
	Recent []ProfileGame
}

const profilePlayerGamesSQL = `
SELECT g.id, g.played_at, g.outcome, g.full_turns, g.red_user,
       CASE WHEN length(o.salt) = 0 AND length(o.hash) = 0 THEN 1 ELSE 0 END,
       o.username,
       CASE WHEN length(o.salt) = 0 AND length(o.hash) = 0 THEN COALESCE(g.bot_name, o.username) ELSE o.username END
FROM games g
JOIN users o ON o.id = CASE WHEN g.red_user = ? THEN g.blue_user ELSE g.red_user END
WHERE g.red_user = ? OR g.blue_user = ?
ORDER BY g.played_at DESC, g.id DESC`

func tierOfAccount(username string) (string, bool) {
	for i := range config.Tiers {
		if username == config.BotAccountName(i) {
			return config.Tiers[i].Name, true
		}
	}
	return "", false
}

func profileResult(won, draw bool) string {
	switch {
	case draw:
		return "draw"
	case won:
		return "win"
	default:
		return "loss"
	}
}

func (s *Store) PlayerProfile(u User) (PlayerProfile, error) {
	rows, err := s.db.Query(profilePlayerGamesSQL, u.ID, u.ID, u.ID)
	if err != nil {
		return PlayerProfile{}, fmt.Errorf("server: player profile %d: %w", u.ID, err)
	}
	defer func() { _ = rows.Close() }()
	pp := PlayerProfile{User: u}
	byTier := make(map[string]*TierRecord)
	for rows.Next() {
		var (
			gameID   int64
			playedAt int64
			redUser  int64
			outcome  string
			turns    int
			botSeat  int
			account  string
			display  string
		)
		if err := rows.Scan(&gameID, &playedAt, &outcome, &turns, &redUser, &botSeat, &account, &display); err != nil {
			return PlayerProfile{}, fmt.Errorf("server: player profile %d: %w", u.ID, err)
		}
		won := (redUser == u.ID && outcome == OutcomeRed) || (redUser != u.ID && outcome == OutcomeBlue)
		draw := outcome == OutcomeDraw
		pp.Games++
		pp.All.add(won, draw)
		if botSeat == 1 {
			pp.Bot.add(won, draw)
			if tier, ok := tierOfAccount(account); ok {
				rec := byTier[tier]
				if rec == nil {
					rec = &TierRecord{Tier: tier}
					byTier[tier] = rec
				}
				rec.add(won, draw)
			}
		} else {
			pp.PvP.add(won, draw)
		}
		if len(pp.Recent) < config.ProfileRecentGames {
			pp.Recent = append(pp.Recent, ProfileGame{
				GameID: gameID, PlayedAt: playedAt,
				Opponent: display, OpponentProfile: "/players/" + url.PathEscape(account),
				Result: profileResult(won, draw), Turns: turns,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return PlayerProfile{}, fmt.Errorf("server: player profile %d: %w", u.ID, err)
	}
	for i := range config.Tiers {
		if rec := byTier[config.Tiers[i].Name]; rec != nil {
			pp.Tiers = append(pp.Tiers, *rec)
		}
	}
	return pp, nil
}

type ProfilePages struct {
	store *Store
}

func NewProfilePages(store *Store) *ProfilePages {
	return &ProfilePages{store: store}
}

var profileTmpl = shellPage("profile.tmpl")

func (p *ProfilePages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /players/{name}", p.handleProfile)
}

type profileGameView struct {
	GameID          int64
	When            string
	Opponent        string
	OpponentProfile string
	Result          string
	Turns           int
}

type profileView struct {
	Me        *shellViewer
	Name      string
	IsBot     bool
	Rating    int
	Games     int
	Wins      int
	Losses    int
	Draws     int
	PvPWins   int
	PvPLosses int
	PvPDraws  int
	BotWins   int
	BotLosses int
	BotDraws  int
	Tiers     []TierRecord
	Recent    []profileGameView
}

func (v profileView) WLD() string {
	return strconv.Itoa(v.Wins) + "W-" + strconv.Itoa(v.Losses) + "L-" + strconv.Itoa(v.Draws) + "D"
}

func (p *ProfilePages) handleProfile(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "profile read failed", http.StatusInternalServerError)
		return
	}
	u, err := p.store.UserByUsername(r.PathValue("name"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "profile read failed", http.StatusInternalServerError)
		return
	}
	pp, err := p.store.PlayerProfile(u)
	if err != nil {
		http.Error(w, "profile read failed", http.StatusInternalServerError)
		return
	}
	rating, err := currentRating(p.store, u.ID)
	if err != nil {
		http.Error(w, "profile read failed", http.StatusInternalServerError)
		return
	}
	view := profileView{
		Me: me, Name: u.Username,
		IsBot:  len(u.Salt) == 0 && len(u.Hash) == 0,
		Rating: rating,
		Games:  pp.Games,
		Wins:   pp.All.Wins, Losses: pp.All.Losses, Draws: pp.All.Draws,
		PvPWins: pp.PvP.Wins, PvPLosses: pp.PvP.Losses, PvPDraws: pp.PvP.Draws,
		BotWins: pp.Bot.Wins, BotLosses: pp.Bot.Losses, BotDraws: pp.Bot.Draws,
		Tiers:  pp.Tiers,
		Recent: make([]profileGameView, 0, len(pp.Recent)),
	}
	for _, g := range pp.Recent {
		view.Recent = append(view.Recent, profileGameView{
			GameID:   g.GameID,
			When:     time.Unix(g.PlayedAt, 0).UTC().Format(historyTimeFormat),
			Opponent: g.Opponent, OpponentProfile: g.OpponentProfile,
			Result: g.Result, Turns: g.Turns,
		})
	}
	renderShell(w, http.StatusOK, profileTmpl, "base", view)
}
