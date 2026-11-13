import { clearAuth, getAuthToken, getAutoLogin, getStoredAuth, saveAuth, tokenIsExpired } from './auth-storage';
import { getLoginUrl } from './auth-redirect';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

let automaticLogin: Promise<LoginResponse | null> | null = null;

// Share a single login across concurrent requests. Renewal never extends 120h.
export function restoreAutomaticLogin(): Promise<LoginResponse | null> {
  if (automaticLogin) return automaticLogin;
  const credentials = getAutoLogin();
  if (!credentials) return Promise.resolve(null);
  const previousToken = getAuthToken();
  const renew = async () => {
    const latest = getAutoLogin();
    if (!latest || latest.expiresAt !== credentials.expiresAt || latest.username !== credentials.username) return null;
    const stored = getStoredAuth();
    if (stored && stored.token !== previousToken && !tokenIsExpired(stored.token)) return stored;
    const response = await fetch(`${API_BASE_URL}/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: credentials.username, password: credentials.password }),
    });
    const data: ApiResponse<LoginResponse> = await response.json();
    // Logout, a different manual login, or the deadline may supersede this request.
    const current = getAutoLogin();
    if (!current || current.expiresAt !== credentials.expiresAt || current.username !== credentials.username) return null;
    if (response.status === 401 || data.code === 401) {
      clearAuth();
      return null;
    }
    if (!response.ok || data.code !== 0 || !data.data) {
      throw new Error(data.message || 'Automatic login failed');
    }
    saveAuth(data.data);
    return data.data;
  };
  // Separate tabs share the same backend token version, so serialize renewals.
  automaticLogin = (async () => {
    if (typeof navigator !== 'undefined' && navigator.locks) {
      return await navigator.locks.request('mtv2-auto-login', renew);
    }
    return renew();
  })().finally(() => { automaticLogin = null; });
  return automaticLogin;
}

interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

// Unique error code returned by the backend when a non-group account's
// points balance is too low to use the search endpoints.
export const CODE_INSUFFICIENT_POINTS = 4290;

// Returned when the 5-hour or weekly search quota is used up. The message
// already states which quota is exhausted and when it resets.
export const CODE_SEARCH_QUOTA_EXCEEDED = 4291;

// ApiError carries the backend's numeric error code alongside the message so
// callers can branch on specific error conditions (e.g. CODE_INSUFFICIENT_POINTS).
export class ApiError extends Error {
  code: number;

  constructor(code: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
  }
}

async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {},
  retried = false
): Promise<T> {
  let token = getAuthToken();
  if (endpoint !== '/login' && endpoint !== '/logout' && !retried && getAutoLogin() && (!token || tokenIsExpired(token))) {
    token = (await restoreAutomaticLogin())?.token || null;
  }
  
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers,
  });

  // Handle an expired login by preserving the current in-app location so the
  // user can return to it after authenticating again. A rejected login request
  // itself should stay on the login page and display the backend error.
  if (response.status === 401 && endpoint !== '/login') {
    if (!retried && endpoint !== '/logout' && getAutoLogin()) {
      // Another concurrent request may already have replaced the rejected token.
      const current = getStoredAuth();
      if ((current && current.token !== token) || await restoreAutomaticLogin()) {
        return apiRequest<T>(endpoint, options, true);
      }
    }
    if (typeof window !== 'undefined') {
      clearAuth();
      window.location.replace(getLoginUrl(true));
    }
    throw new Error('Unauthorized');
  }

  // Try to parse JSON, but handle cases where response might not be JSON
  let data: ApiResponse<T>;
  try {
    data = await response.json();
  } catch (error) {
    // If response is not JSON, throw a generic error
    throw new Error('Invalid response from server');
  }

  if (data.code !== 0) {
    throw new ApiError(data.code, data.message || 'Request failed');
  }

  return data.data as T;
}

// Auth API
export interface User {
  id: string;
  username: string;
  email: string;
  points: number;
  is_admin: boolean;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export const authAPI = {
  login: async (credentials: { username: string; password: string }): Promise<LoginResponse> => {
    return apiRequest<LoginResponse>('/login', {
      method: 'POST',
      body: JSON.stringify(credentials),
    });
  },

  register: async (data: { username: string; email: string; password: string; invite_code: string }): Promise<void> => {
    return apiRequest<void>('/register', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  logout: async (): Promise<void> => {
    return apiRequest<void>('/logout', {
      method: 'POST',
    });
  },

  changePassword: async (data: { old_password: string; new_password: string }): Promise<void> => {
    return apiRequest<void>('/change-password', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },
};

// Invitation API
export interface Invitation {
  invite_code: string;
  created_at: string;
  used: boolean;
  used_by?: string | null;
}

export interface InvitationListResponse {
  invitations: Invitation[];
  points: number;
}

export interface InvitationGenerateResponse {
  count: number;
  invitations: Invitation[];
}

export const invitationAPI = {
  list: async (): Promise<InvitationListResponse> => {
    return apiRequest<InvitationListResponse>('/invitations');
  },

  generate: async (data: { count: number }): Promise<InvitationGenerateResponse> => {
    return apiRequest<InvitationGenerateResponse>('/invitations/generate', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },
};

// Problem API
export interface ProblemUploadResponse {
  message: string;
}

export interface ProblemUploadMultipleResponse {
  total: number;
  success: number;
  failed: number;
  results: Array<{
    userId: string;
    taskId: string;
    success: boolean;
    message: string;
  }>;
  uploadIP: string;
}

export const problemAPI = {
  upload: async (data: { userId: string; taskId: string }): Promise<ProblemUploadResponse> => {
    return apiRequest<ProblemUploadResponse>('/problem/upload', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  uploadMultiple: async (data: { problems: Array<{ userId: string; taskId: string }> }): Promise<ProblemUploadMultipleResponse> => {
    return apiRequest<ProblemUploadMultipleResponse>('/problem/upload-multiple', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  count: async (): Promise<{ counts: { elasticsearch: number; mongodb: number; redis: number } }> => {
    return apiRequest<{ counts: { elasticsearch: number; mongodb: number; redis: number } }>('/problem/count');
  },

  uploadDaily: async (data: { userId: string; dateId: string }): Promise<ProblemUploadResponse> => {
    return apiRequest<ProblemUploadResponse>('/problem/upload_daily', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  refreshComments: async (mongoId: string): Promise<ProblemUploadResponse> => {
    return apiRequest<ProblemUploadResponse>(`/problem/refresh-comments/${encodeURIComponent(mongoId)}`, {
      method: 'POST',
    });
  },
};

export interface AccountPointsResponse {
  accountId: string;
  username: string;
  totalPoints: number;
  weeklyPoints: number;
  year: number;
  weekId: number;
}

export const accountAPI = {
  getAvatarUrl: (accountId: string): string => {
    return `${API_BASE_URL}/account/avatar?id=${encodeURIComponent(accountId)}`;
  },

  getAccountPoints: async (accountId: string): Promise<AccountPointsResponse> => {
    return apiRequest<AccountPointsResponse>(`/user_detail/account_points?accountId=${encodeURIComponent(accountId)}`);
  },
};

// Search API
export interface SearchResult {
  id: any; // ESID
  mongo_id: string;
  user_review: string;
  timestamp: number;
  answer: number;
  hot1_answer?: number | null;
  comment_ratio?: number | null;
  ratio_1: number;
  ratio_2: number;
  _score: number;
  _highlight?: {
    [key: string]: string[];
  };
}

export interface SearchResponse {
  total: number;
  results: SearchResult[];
}

export interface SearchQuotaWindow {
  limit: number;
  used: number;
  remaining: number;
  started_at?: string;
  reset_at: string | null; // null: the 5-hour window has not started yet
  reset_at_unix: number | null;
}

export interface SearchQuotaStatus {
  unlimited: boolean; // admin / group accounts have no quota
  last_week_score: number;
  five_hour?: SearchQuotaWindow;
  weekly?: SearchQuotaWindow;
  next_reset_at: string | null;
  next_reset_at_unix: number | null;
  blocked: boolean;
  unlock_at: string | null;
  unlock_at_unix: number | null;
  exhausted_windows?: Array<'five_hour' | 'weekly'>;
}

export interface NotesSearchItem {
  text: string;
  answer: number;
  reply: string;
}

export interface ProblemUploadersResponse {
  uploader: string;
  uploader_name: string;
  previous_uploaders: string[];
  previous_uploaders_name: string[];
}

export const searchAPI = {
  search: async (keyword: string, limit: number = 15): Promise<SearchResponse> => {
    const params = new URLSearchParams({
      keyword,
      limit: limit.toString(),
    });
    return apiRequest<SearchResponse>(`/problem/search?${params.toString()}`);
  },

  // Read-only: never consumes search quota.
  quota: async (): Promise<SearchQuotaStatus> => {
    return apiRequest<SearchQuotaStatus>('/problem/quota');
  },

  recent: async (limit: number = 15): Promise<SearchResponse> => {
    const params = new URLSearchParams({
      limit: limit.toString(),
    });
    return apiRequest<SearchResponse>(`/problem/recent?${params.toString()}`);
  },

  notesSearch: async (keyword: string, limit: number = 5): Promise<NotesSearchItem[]> => {
    const params = new URLSearchParams({
      keyword,
      limit: limit.toString(),
    });
    return apiRequest<NotesSearchItem[]>(`/notes/search?${params.toString()}`);
  },

  getByMongoId: async (mongoId: string, blockMaliciousComment: number = 1): Promise<any> => {
    const params = new URLSearchParams({
      blockMaliciousComment: blockMaliciousComment.toString(),
    });
    return apiRequest<any>(`/problem/by-mongoid/${mongoId}?${params.toString()}`);
  },

  getProblemUploaders: async (mongoId: string): Promise<ProblemUploadersResponse> => {
    return apiRequest<ProblemUploadersResponse>(`/problem/uploaders/${mongoId}`);
  },

  getProblemComments: async (problemId: string, ignore: number = 1): Promise<ProblemCommentsResponse> => {
    const params = new URLSearchParams({ ignore: ignore.toString() });
    return apiRequest<ProblemCommentsResponse>(`/problem/comments/${problemId}?${params.toString()}`);
  },
};

export interface ProblemReply {
  replyid: string;
  userid: number;
  name: string;
  likes: number;
  timestamp: number;
}

export interface ProblemCommentAudio {
  url: string;
  duration: number;
  audioText: string;
}

export interface ProblemComment {
  id: string;
  choice: number;
  content: string;
  likes: number;
  name: string;
  timestamp: number;
  userid: number;
  images: string[];
  audios: ProblemCommentAudio[];
  replies: ProblemReply[];
  locationInfo: string | null;
}

export interface ProblemCommentsResponse {
  data: ProblemComment[];
}

// Media API
export interface MediaHashResponse {
  url: string;
  hash: string;
}

export const mediaAPI = {
  generateHash: async (url: string): Promise<MediaHashResponse> => {
    return apiRequest<MediaHashResponse>('/media/hash', {
      method: 'POST',
      body: JSON.stringify({ url }),
    });
  },

  getImageUrl: async (imageUrl: string): Promise<string> => {
    // First get the hash
    const { hash } = await mediaAPI.generateHash(imageUrl);
    // Return the proxied URL
    const params = new URLSearchParams({
      url: imageUrl,
      hash: hash,
    });
    return `${API_BASE_URL}/media/image?${params.toString()}`;
  },

  getVideoUrl: async (videoUrl: string): Promise<string> => {
    // First get the hash
    const { hash } = await mediaAPI.generateHash(videoUrl);
    // Return the proxied URL
    const params = new URLSearchParams({
      url: videoUrl,
      hash: hash,
    });
    return `${API_BASE_URL}/media/video?${params.toString()}`;
  },

  getAudioUrl: async (audioUrl: string): Promise<string> => {
    // First get the hash
    const { hash } = await mediaAPI.generateHash(audioUrl);
    // Return the proxied URL
    const params = new URLSearchParams({
      url: audioUrl,
      hash: hash,
    });
    return `${API_BASE_URL}/media/audio?${params.toString()}`;
  },
};

// User Detail API
export interface UserInfoResponse {
  userName: string;
  likes: number;
  replies: number;
  malicious: boolean;
}

export interface UserComment {
  id: string;
  problemId: string;
  commentId: string;
  userId: string;
  userName: string;
  userPic: string;
  createTime: number;
  content: string;
  approveCount: number;
  replyTotal: number;
  isAnonymous: boolean;
  voteOperate: string;
  choice: number;
  images: string[];
  audios: ProblemCommentAudio[];
}

export interface UserCommentsResponse {
  comments: UserComment[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface VanishedComment {
  problemId: string;
  user_review: string;
  timestamp: string;
  commentId: string;
  content: string;
  likes: number;
  createTime: string;
  url: string;
}

export interface RankingItem {
  userId: string;
  userName: string;
  likes: number;
  commentCount: number;
  rank: number;
}

export interface RankingsResponse {
  rankings: RankingItem[];
  total: number;
  page: number;
  pageSize: number;
  totalPages: number;
}

export interface UserSearchResultItem {
  userId: string;
  userName: string;
}

export interface UserSearchResponse {
  users: UserSearchResultItem[];
}

// AI user summary ("犀利点评 + 用户画像")
export interface AIUserSummaryGuess {
  value: string;
  confidence: string;
  disclaimer: string;
}

export interface AIUserSummaryProfile {
  summary: string;
  expressionStyle: string[];
  opinionTendency: string[];
  interactionPattern: string[];
  genderGuess: AIUserSummaryGuess;
  mbtiGuess: AIUserSummaryGuess;
}

export interface AIUserSummaryEvidence {
  claim: string;
  evidenceIds: string[];
  reason: string;
}

export interface AIUserSummaryResult {
  roast: string;
  profile: AIUserSummaryProfile;
  evidence: AIUserSummaryEvidence[];
  limitations: string[];
  /** 1-5 comment quality stars; absent/null on older summaries */
  commentQualityStars?: number | null;
}

export type AIUserSummaryStatus = 'none' | 'ready' | 'failed';

export interface AIUserSummaryResponse {
  status: AIUserSummaryStatus;
  result?: AIUserSummaryResult;
  provider?: string;
  model?: string;
  promptVersion?: string;
  generatedAt?: number; // unix seconds
  expiresAt?: number; // unix seconds
  stale: boolean;
  lastError?: string;
}

export const userDetailAPI = {
  getAvatarUrl: (userId: string): string => {
    const token = getAuthToken();
    if (!token) return '';
    return `${API_BASE_URL}/user_detail/avatar?userId=${userId}&token=${encodeURIComponent(token)}`;
  },

  getUserInfo: async (userId: string): Promise<UserInfoResponse> => {
    return apiRequest<UserInfoResponse>(`/user_detail/user_info?userId=${userId}`);
  },

  getComments: async (userId: string, page: number = 1, limit: number = 10): Promise<UserCommentsResponse> => {
    const params = new URLSearchParams({
      userId,
      page: page.toString(),
      limit: limit.toString(),
    });
    return apiRequest<UserCommentsResponse>(`/user_detail/comments?${params.toString()}`);
  },

  getVanishedComments: async (userId: string): Promise<VanishedComment[]> => {
    return apiRequest<VanishedComment[]>(`/user_detail/vanished/${encodeURIComponent(userId)}`);
  },

  getRankings: async (page: number = 1): Promise<RankingsResponse> => {
    return apiRequest<RankingsResponse>(`/user_detail/rankings?page=${page}`);
  },

  searchUsers: async (keyword: string, limit: number = 10): Promise<UserSearchResponse> => {
    const params = new URLSearchParams({ keyword, limit: limit.toString() });
    return apiRequest<UserSearchResponse>(`/user_detail/search_users?${params.toString()}`);
  },

  toggleMalicious: async (userId: string): Promise<{ malicious: boolean; message: string }> => {
    return apiRequest<{ malicious: boolean; message: string }>(`/user_detail/toggle_malicious?userId=${userId}`, {
      method: 'POST',
    });
  },

  getAISummary: async (userId: string): Promise<AIUserSummaryResponse> => {
    return apiRequest<AIUserSummaryResponse>(`/user_detail/ai_summary?userId=${userId}`);
  },

  generateAISummary: async (userId: string): Promise<AIUserSummaryResponse> => {
    return apiRequest<AIUserSummaryResponse>(`/user_detail/ai_summary?userId=${userId}`, {
      method: 'POST',
    });
  },
};

// Points leaderboard API
export interface LeaderboardEntry {
  id: string;
  username: string;
  points: number;
  score: number | null;
}

export interface LeaderboardResponse {
  year: number;
  weekId: number;
  rankings: LeaderboardEntry[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

export const pointsAPI = {
  getLeaderboard: async (page: number = 1, limit: number = 50): Promise<LeaderboardResponse> => {
    const params = new URLSearchParams({
      page: page.toString(),
      limit: limit.toString(),
    });
    return apiRequest<LeaderboardResponse>(`/points/leaderboard?${params.toString()}`);
  },
};

// Comment word clouds use immutable snapshots so previews and pages agree.
export type WordCloudRange = '24h' | '7d';
export interface WordCloudComment {
  id: string;
  content: string;
  createTime: number;
  likes: number;
  name: string;
  isAnonymous: boolean;
}
export interface WordCloudWord {
  text: string;
  count: number;
  preview: WordCloudComment[];
}
export interface WordCloudResponse {
  snapshot: string;
  generatedAt: number;
  expiresAt: number;
  range: WordCloudRange;
  from: number;
  to: number;
  commentCount: number;
  words: WordCloudWord[];
}
export interface WordCloudCommentsResponse {
  snapshot: string;
  word: string;
  comments: WordCloudComment[];
  total: number;
  page: number;
  totalPages: number;
}
export const statsAPI = {
  wordCloud: (range: WordCloudRange, signal?: AbortSignal) =>
    apiRequest<WordCloudResponse>(`/stats/wordcloud?range=${range}`, { signal }),
  comments: (range: WordCloudRange, snapshot: string, word: string, page: number, signal?: AbortSignal) => {
    const params = new URLSearchParams({ range, snapshot, word, page: String(page) });
    return apiRequest<WordCloudCommentsResponse>(`/stats/wordcloud/comments?${params}`, { signal });
  },
};
