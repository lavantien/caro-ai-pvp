package server

import "github.com/lavantien/caro-ai-pvp/internal/config"

func NewBotSearcherFor(variant string, t config.Tier) Searcher { return newBotSearcherFor(variant, t) }
