const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || '/api';

interface ApiResponse<T> {
  code: number;
  message: string;
  data?: T;
}

async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
  
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

  // Handle 401 Unauthorized - redirect to login
  if (response.status === 401) {
    // Clear auth data
    if (typeof window !== 'undefined') {
      localStorage.removeItem('token');
      localStorage.removeItem('user');
      // Redirect to login with expired parameter
      window.location.href = '/login?expired=true';
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
    throw new Error(data.message || 'Request failed');
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
};

// Search API
export interface SearchResult {
  id: any; // ESID
  mongo_id: string;
  user_review: string;
  timestamp: number;
  answer: number;
  hot1_answer?: number | null;
  comment_answer?: number | null;
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

export interface NotesSearchItem {
  text: string;
  answer: number;
}

export interface NotesSearchResponse {
  data: NotesSearchItem[];
}

export const searchAPI = {
  search: async (keyword: string, limit: number = 15): Promise<SearchResponse> => {
    const params = new URLSearchParams({
      keyword,
      limit: limit.toString(),
    });
    return apiRequest<SearchResponse>(`/problem/search?${params.toString()}`);
  },

  recent: async (limit: number = 15): Promise<SearchResponse> => {
    const params = new URLSearchParams({
      limit: limit.toString(),
    });
    return apiRequest<SearchResponse>(`/problem/recent?${params.toString()}`);
  },

  notesSearch: async (keyword: string, limit: number = 5): Promise<NotesSearchResponse> => {
    const params = new URLSearchParams({
      keyword,
      limit: limit.toString(),
    });
    return apiRequest<NotesSearchResponse>(`/notes/search?${params.toString()}`);
  },

  getByMongoId: async (mongoId: string, blockMaliciousComment: number = 1): Promise<any> => {
    const params = new URLSearchParams({
      blockMaliciousComment: blockMaliciousComment.toString(),
    });
    return apiRequest<any>(`/problem/by-mongoid/${mongoId}?${params.toString()}`);
  },
};

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
}

export interface UserCommentsResponse {
  comments: UserComment[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface RankingItem {
  userId: string;
  userName: string;
  likes: number;
  commentCount: number;
}

export interface RankingsResponse {
  rankings: RankingItem[];
  total: number;
}

export const userDetailAPI = {
  getAvatarUrl: (userId: string): string => {
    const token = typeof window !== 'undefined' ? localStorage.getItem('token') : null;
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

  getRankings: async (): Promise<RankingsResponse> => {
    return apiRequest<RankingsResponse>('/user_detail/rankings');
  },

  toggleMalicious: async (userId: string): Promise<{ malicious: boolean; message: string }> => {
    return apiRequest<{ malicious: boolean; message: string }>(`/user_detail/toggle_malicious?userId=${userId}`, {
      method: 'POST',
    });
  },
};
