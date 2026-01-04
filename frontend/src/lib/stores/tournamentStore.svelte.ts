import * as signalR from "@microsoft/signalr";

type TournamentStatus = "idle" | "running" | "paused" | "completed";
type Player = "none" | "red" | "blue";
type AIDifficulty = "beginner" | "easy" | "normal" | "medium" | "hard" | "harder" | "veryHard" | "expert" | "master" | "grandmaster" | "legend";

function mapStatusFromBackend(status: number | string): TournamentStatus {
  if (typeof status === "string") return status as TournamentStatus;
  const statusMap: Record<number, TournamentStatus> = { 0: "idle", 1: "running", 2: "paused", 3: "completed" };
  return statusMap[status] ?? "idle";
}

function mapDifficultyFromBackend(diff: number | string): AIDifficulty {
  if (typeof diff === "string") return diff as AIDifficulty;
  const diffMap: Record<number, AIDifficulty> = { 1: "beginner", 2: "easy", 3: "normal", 4: "medium", 5: "hard", 6: "harder", 7: "veryHard", 8: "expert", 9: "master", 10: "grandmaster", 11: "legend" };
  return diffMap[diff] ?? "beginner";
}

export interface BoardCell { x: number; y: number; player: string; }
export interface AIBot { name: string; difficulty: AIDifficulty; elo: number; wins: number; losses: number; draws: number; gamesPlayed: number; winRate: number; }
export interface EngineStats { depthAchieved: number; nodesSearched: number; nodesPerSecond: number; tableHitRate: number; ponderingActive: boolean; vcfDepthAchieved: number; vcfNodesSearched: number; }
export interface CurrentMatchInfo { gameId: string; redBotName: string; blueBotName: string; redDifficulty: AIDifficulty; blueDifficulty: AIDifficulty; moveNumber: number; board: BoardCell[]; redTimeRemainingMs: number; blueTimeRemainingMs: number; initialTimeSeconds: number; incrementSeconds: number; lastMove: { x: number; y: number } | null; lastMoveTimestamp: number; redLastStats: EngineStats | null; blueLastStats: EngineStats | null; redPondering: boolean; bluePondering: boolean; currentThinkingStats: ThinkingStats | null; }
export interface ThinkingStats { player: string; currentDepth: number; nodesSearched: number; nodesPerSecond: number; tableHitRate: number; vcfDepthAchieved: number; vcfNodesSearched: number; elapsedMs: number; }
export interface TournamentProgress { completed: number; total: number; percent: number; }
export interface MatchResult { winner: Player; loser: Player; totalMoves: number; durationMs: number; winnerDifficulty: AIDifficulty; loserDifficulty: AIDifficulty; isDraw: boolean; endedByTimeout: boolean; winnerBotName?: string; loserBotName?: string; }
export interface TournamentState { status: TournamentStatus; progress: TournamentProgress; bots: AIBot[]; matchHistory: MatchResult[]; currentMatch: CurrentMatchInfo | null; startTimeUtc: string; endTimeUtc: string | null; elapsed: string; connectionState: "disconnected" | "connecting" | "connected" | "reconnecting"; errorMessage: string | null; gameLogs: GameLogEntry[]; }

export interface GameLogEntry { timestamp: string; level: string; source: string; message: string; }

const API_BASE = "http://localhost:5207/api/tournament";
const HUB_URL = "http://localhost:5207/hubs/tournament";

export class TournamentStore {
  state = $state<TournamentState>({ status: "idle", progress: { completed: 0, total: 0, percent: 0 }, bots: [], matchHistory: [], currentMatch: null, startTimeUtc: "", endTimeUtc: null, elapsed: "", connectionState: "disconnected", errorMessage: null, gameLogs: [] });
  private connection: signalR.HubConnection | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private countdownInterval: ReturnType<typeof setInterval> | null = null;

  constructor() { this.countdownInterval = setInterval(() => this.updateCountdown(), 100); }

  private updateCountdown(): void {
    if (!this.state.currentMatch || this.state.status !== "running") return;
    const now = Date.now();
    // Calculate elapsed time since last move timestamp
    const elapsed = now - this.state.currentMatch.lastMoveTimestamp;
    const isRedTurn = this.state.currentMatch.moveNumber % 2 === 0;
    if (isRedTurn) this.state.currentMatch.redTimeRemainingMs = Math.max(0, this.state.currentMatch.redTimeRemainingMs - elapsed);
    else this.state.currentMatch.blueTimeRemainingMs = Math.max(0, this.state.currentMatch.blueTimeRemainingMs - elapsed);
    // Note: Don't update lastMoveTimestamp here - it should only be updated in OnBoardUpdate
    // This prevents race conditions with board state updates
  }

  async connect(): Promise<void> {
    if (this.connection?.state === signalR.HubConnectionState.Connected) return;
    this.state.connectionState = "connecting";
    this.state.errorMessage = null;
    try {
      this.connection = new signalR.HubConnectionBuilder().withUrl(HUB_URL, { skipNegotiation: false, withCredentials: true }).withAutomaticReconnect({ nextRetryDelayInMilliseconds: (retryContext) => { if (retryContext.previousRetryCount >= this.maxReconnectAttempts) return null; return Math.min(1000 * Math.pow(2, retryContext.previousRetryCount), 10000); } }).configureLogging(signalR.LogLevel.Information).build();
      this.setupEventHandlers();
      this.connection.onreconnecting((error) => { this.state.connectionState = "reconnecting"; this.state.errorMessage = error?.message || "Connection lost. Reconnecting..."; });
      this.connection.onreconnected((connectionId) => { this.state.connectionState = "connected"; this.state.errorMessage = null; this.fetchState(); });
      this.connection.onclose((error) => { this.state.connectionState = "disconnected"; if (error) this.state.errorMessage = "Connection closed: " + error.message; });
      await this.connection.start();
      this.state.connectionState = "connected";
      this.reconnectAttempts = 0;
      await this.fetchState();
    } catch (err) { this.state.connectionState = "disconnected"; this.state.errorMessage = "Failed to connect: " + (err as Error).message; throw err; }
  }

  async disconnect(): Promise<void> {
    if (this.countdownInterval) { clearInterval(this.countdownInterval); this.countdownInterval = null; }
    if (this.connection) { await this.connection.stop(); this.connection = null; }
    this.state.connectionState = "disconnected";
  }

  private setupEventHandlers(): void {
    if (!this.connection) return;
    // Remove any existing handlers first to prevent duplicates (e.g., from HMR in dev mode)
    this.connection.off("OnGameStarted");
    this.connection.off("OnBoardUpdate");
    this.connection.off("OnMovePlayed");
    this.connection.off("OnThinkingStats");
    this.connection.off("OnPonderingStatus");
    this.connection.off("OnGameFinished");
    this.connection.off("OnTournamentProgress");
    this.connection.off("OnTournamentCompleted");
    this.connection.off("OnTournamentStatusChanged");
    this.connection.off("OnELOUpdated");
    this.connection.off("OnGameLog");

    this.connection.on("OnGameStarted", (gameId: string, redBot: string, blueBot: string, redDiff: AIDifficulty, blueDiff: AIDifficulty) => {
      const now = Date.now();
      this.state.currentMatch = { gameId, redBotName: redBot, blueBotName: blueBot, redDifficulty: redDiff, blueDifficulty: blueDiff, moveNumber: 0, board: [], redTimeRemainingMs: 420000, blueTimeRemainingMs: 420000, initialTimeSeconds: 420, incrementSeconds: 5, lastMove: null, lastMoveTimestamp: now, redLastStats: null, blueLastStats: null, redPondering: false, bluePondering: false, currentThinkingStats: null };
    });
    // OnBoardUpdate now includes move info for atomic updates
    this.connection.on("OnBoardUpdate", (gameId: string, boardCells: BoardCell[], moveNumber: number, player: string, x: number, y: number) => {
      if (this.state.currentMatch && this.state.currentMatch.gameId === gameId) {
        // Update all board-related state atomically, including timestamp
        // This prevents race conditions between board state and time tracking
        const now = Date.now();
        this.state.currentMatch.board = boardCells;
        this.state.currentMatch.moveNumber = moveNumber;
        if (x >= 0 && y >= 0) {
          this.state.currentMatch.lastMove = { x, y };
        }
        // Update timestamp here to ensure sync with board state
        this.state.currentMatch.lastMoveTimestamp = now;
        // Clear thinking stats when move is recorded
        this.state.currentMatch.currentThinkingStats = null;
      }
    });
    this.connection.on("OnMovePlayed", (moveEvent: any) => {
      if (!this.state.currentMatch) return;
      // Update times from backend (authoritative time source)
      this.state.currentMatch.redTimeRemainingMs = moveEvent.redTimeRemainingMs;
      this.state.currentMatch.blueTimeRemainingMs = moveEvent.blueTimeRemainingMs;
      // Note: lastMoveTimestamp is updated in OnBoardUpdate to maintain sync with board state
      // Create new object for each player to avoid reference sharing
      const stats: EngineStats = {
        depthAchieved: moveEvent.depthAchieved ?? 0,
        nodesSearched: moveEvent.nodesSearched ?? 0,
        nodesPerSecond: moveEvent.nodesPerSecond ?? 0,
        tableHitRate: moveEvent.tableHitRate ?? 0,
        ponderingActive: moveEvent.ponderingActive ?? false,
        vcfDepthAchieved: moveEvent.vcfDepthAchieved ?? 0,
        vcfNodesSearched: moveEvent.vcfNodesSearched ?? 0
      };
      // Store stats for the player who made this move (clone to prevent reference sharing)
      if (moveEvent.player === "red") {
        this.state.currentMatch.redLastStats = { ...stats };
        this.state.currentMatch.redPondering = moveEvent.ponderingActive ?? false;
        this.state.currentMatch.bluePondering = false;
      } else {
        this.state.currentMatch.blueLastStats = { ...stats };
        this.state.currentMatch.bluePondering = moveEvent.ponderingActive ?? false;
        this.state.currentMatch.redPondering = false;
      }
    });
    // Real-time thinking stats during AI search
    this.connection.on("OnThinkingStats", (statsEvent: any) => {
      if (!this.state.currentMatch || this.state.currentMatch.gameId !== statsEvent.gameId) return;
      this.state.currentMatch.currentThinkingStats = {
        player: statsEvent.player,
        currentDepth: statsEvent.currentDepth,
        nodesSearched: statsEvent.nodesSearched,
        nodesPerSecond: statsEvent.nodesPerSecond,
        tableHitRate: statsEvent.tableHitRate,
        vcfDepthAchieved: statsEvent.vcfDepthAchieved,
        vcfNodesSearched: statsEvent.vcfNodesSearched,
        elapsedMs: statsEvent.elapsedMs
      };
    });
    // Pondering status updates
    this.connection.on("OnPonderingStatus", (statusEvent: any) => {
      if (!this.state.currentMatch || this.state.currentMatch.gameId !== statusEvent.gameId) return;
      if (statusEvent.player === "red") {
        this.state.currentMatch.redPondering = statusEvent.isPondering;
      } else if (statusEvent.player === "blue") {
        this.state.currentMatch.bluePondering = statusEvent.isPondering;
      }
    });
    this.connection.on("OnGameFinished", (finished: any) => {
      if (!this.state.currentMatch) return;
      const isRedWinner = finished.winner === "red";
      this.state.matchHistory = [{ winner: finished.winner, loser: finished.loser, totalMoves: finished.totalMoves, durationMs: finished.durationMs, winnerDifficulty: isRedWinner ? this.state.currentMatch.redDifficulty : this.state.currentMatch.blueDifficulty, loserDifficulty: isRedWinner ? this.state.currentMatch.blueDifficulty : this.state.currentMatch.redDifficulty, isDraw: finished.isDraw, endedByTimeout: finished.endedByTimeout, winnerBotName: isRedWinner ? this.state.currentMatch.redBotName : this.state.currentMatch.blueBotName, loserBotName: isRedWinner ? this.state.currentMatch.blueBotName : this.state.currentMatch.redBotName }, ...this.state.matchHistory.slice(0, 19)];
    });
    this.connection.on("OnTournamentProgress", (completed: number, total: number, percent: number, currentMatch: string) => { this.state.progress = { completed, total, percent }; });
    this.connection.on("OnTournamentCompleted", (finalStandings: AIBot[], totalGames: number, durationMs: number) => { this.state.status = "completed"; this.state.bots = finalStandings; this.state.progress = { completed: totalGames, total: totalGames, percent: 100 }; this.state.endTimeUtc = new Date().toISOString(); this.state.elapsed = formatDuration(durationMs); });
    this.connection.on("OnTournamentStatusChanged", (status: number | string, message: string) => { this.state.status = mapStatusFromBackend(status); if (this.state.status === "running" && !this.state.startTimeUtc) this.state.startTimeUtc = new Date().toISOString(); });
    this.connection.on("OnELOUpdated", (bots: AIBot[]) => { this.state.bots = bots.sort((a, b) => b.elo - a.elo); });
    this.connection.on("OnGameLog", (logEntry: any) => {
      // Keep last 100 log entries
      this.state.gameLogs = [logEntry, ...this.state.gameLogs].slice(0, 100);
    });
  }

  async fetchState(): Promise<void> {
    try {
      const response = await fetch(API_BASE + "/state");
      if (!response.ok) throw new Error("Failed to fetch state");
      const data = await response.json();
      this.state.status = mapStatusFromBackend(data.status);
      this.state.progress = { completed: data.completedGames, total: data.totalGames, percent: data.progressPercent };
      this.state.bots = (data.bots || []).sort((a: AIBot, b: AIBot) => b.elo - a.elo);
      this.state.matchHistory = (data.matchHistory || []).map((m: any) => ({ winner: m.winner, loser: m.loser, totalMoves: m.totalMoves, durationMs: m.durationMs, winnerDifficulty: mapDifficultyFromBackend(m.winnerDifficulty), loserDifficulty: mapDifficultyFromBackend(m.loserDifficulty), isDraw: m.isDraw, endedByTimeout: m.endedByTimeout, winnerBotName: m.winnerBotName, loserBotName: m.loserBotName }));
      this.state.currentMatch = data.currentMatch ? {
        ...data.currentMatch,
        redDifficulty: mapDifficultyFromBackend(data.currentMatch.redDifficulty),
        blueDifficulty: mapDifficultyFromBackend(data.currentMatch.blueDifficulty),
        redLastStats: data.currentMatch.redLastStats ? { ...data.currentMatch.redLastStats } : null,
        blueLastStats: data.currentMatch.blueLastStats ? { ...data.currentMatch.blueLastStats } : null,
        redPondering: data.currentMatch.redPondering ?? false,
        bluePondering: data.currentMatch.bluePondering ?? false,
        currentThinkingStats: data.currentMatch.currentThinkingStats ?? null
      } : null;
      this.state.startTimeUtc = data.startTimeUtc || "";
      this.state.endTimeUtc = data.endTimeUtc || null;
    } catch (err) { console.error("Failed to fetch:", err); }
  }

  async startTournament(): Promise<boolean> {
    try {
      const response = await fetch(API_BASE + "/start", { method: "POST" });
      if (!response.ok) { const data = await response.json(); this.state.errorMessage = data.message || "Failed to start"; return false; }
      await this.fetchState();
      return true;
    } catch (err) { this.state.errorMessage = "Failed to start: " + (err as Error).message; return false; }
  }

  async pauseTournament(): Promise<boolean> {
    try { const response = await fetch(API_BASE + "/pause", { method: "POST" }); if (!response.ok) return false; await this.fetchState(); return true; } catch (err) { this.state.errorMessage = "Failed to pause: " + (err as Error).message; return false; }
  }

  async resumeTournament(): Promise<boolean> {
    try { const response = await fetch(API_BASE + "/resume", { method: "POST" }); if (!response.ok) return false; await this.fetchState(); return true; } catch (err) { this.state.errorMessage = "Failed to resume: " + (err as Error).message; return false; }
  }

  get sortedBots(): AIBot[] { return [...this.state.bots].sort((a, b) => b.elo - a.elo); }
  get recentMatches(): MatchResult[] { return this.state.matchHistory.slice(0, 50); }
  formatTime(ms: number): string { const seconds = Math.floor(ms / 1000); const minutes = Math.floor(seconds / 60); return minutes + ":" + (seconds % 60).toString().padStart(2, "0"); }
  formatELOChange(bot: AIBot): string { const change = bot.elo - 600; return change >= 0 ? "+" + change : String(change); }
  getELOChangeClass(bot: AIBot): string { const change = bot.elo - 600; if (change > 0) return "text-green-600"; if (change < 0) return "text-red-600"; return "text-gray-500"; }
}

function formatDuration(ms: number): string {
  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  if (hours > 0) return hours + "h " + (minutes % 60) + "m";
  return minutes + "m " + (seconds % 60) + "s";
}

export const tournamentStore = new TournamentStore();
