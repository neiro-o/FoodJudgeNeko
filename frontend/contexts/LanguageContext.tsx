'use client';

import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';

export type Language = 'en' | 'zh';

interface LanguageContextType {
  language: Language;
  setLanguage: (lang: Language) => void;
  t: (key: string, params?: Record<string, string | number>) => string;
}

const LanguageContext = createContext<LanguageContextType | undefined>(undefined);

// Translation keys
const translations: Record<Language, Record<string, string>> = {
  en: {
    // Common
    'welcome': 'Welcome',
    'loading': 'Loading...',
    'logout': 'Logout',
    'language': 'Language',
    'english': 'English',
    'chinese': '中文',
    'nav.home': 'Home',
    'nav.problems': 'Problems',
    'nav.userStats': 'User Details',

    // Page titles
    'pageTitle.home': 'You lose a heart.',
    'pageTitle.login': 'Login',
    'pageTitle.register': 'Register',
    'pageTitle.problems': 'Problems',
    'pageTitle.user': 'User Center',
    
    // Home page
    'home.title': '掉心心了',
    'home.subtitle': '选择与结果不一致，掉小心心了！',
    
    // Login page
    'login.title': 'Welcome Back',
    'login.subtitle': 'Sign in to your account',
    'login.username': 'Username',
    'login.usernamePlaceholder': 'Enter your username',
    'login.password': 'Password',
    'login.passwordPlaceholder': 'Enter your password',
    'login.submit': 'Sign In',
    'login.submitting': 'Signing in...',
    'login.error': 'Login failed. Please try again.',
    'login.noAccount': "Don't have an account?",
    'login.signUp': 'Sign up',
    'login.success': 'Account created successfully! Please sign in.',
    'login.expired': 'Login status deactivated, please login again.',
    
    // Register page
    'register.title': 'Create Account',
    'register.subtitle': 'Sign up with an invitation code',
    'register.inviteCode': 'Invitation Code',
    'register.inviteCodePlaceholder': 'Enter your invitation code',
    'register.username': 'Username',
    'register.usernamePlaceholder': 'Choose a username',
    'register.email': 'Email',
    'register.emailPlaceholder': 'Enter your email',
    'register.password': 'Password',
    'register.passwordPlaceholder': 'Enter your password (min. 6 characters)',
    'register.confirmPassword': 'Confirm Password',
    'register.confirmPasswordPlaceholder': 'Confirm your password',
    'register.submit': 'Create Account',
    'register.submitting': 'Creating Account...',
    'register.error': 'Registration failed. Please try again.',
    'register.passwordMismatch': 'Passwords do not match',
    'register.passwordTooShort': 'Password must be at least 6 characters',
    'register.hasAccount': 'Already have an account?',
    'register.signIn': 'Sign in',
    'register.notice': 'This website is exclusively for large model developers. Abuse of real public reviews (such as directly applying AI opinions for reviews) is not supported. Registration requires an invitation code.',
    
    // Problems page
    'problems.title': 'Problem Lib',
    'problems.userCenter': 'User Center',
    'problems.welcome': 'Welcome, {username}',
    'problems.upload.title': 'Upload Problems',
    'problems.upload.single': 'Single Upload',
    'problems.upload.multiple': 'Multiple Upload',
    'problems.upload.url': 'URL',
    'problems.upload.urlPlaceholder': 'Enter URL...',
    'problems.upload.userId': 'User ID',
    'problems.upload.taskId': 'Task ID',
    'problems.upload.submit': 'Upload Problem',
    'problems.upload.submitting': 'Uploading...',
    'problems.upload.multipleUrls': 'URLs (one per line or separated by commas, tabs, etc.)',
    'problems.upload.multipleUrlsPlaceholder': 'Paste URLs here (separated by newlines, commas, tabs, etc.)...',
    'problems.upload.userIdsFound': 'User IDs found',
    'problems.upload.taskIdsFound': 'Task IDs found',
    'problems.upload.validPairs': 'Valid pairs (after removing duplicates)',
    'problems.upload.warning': '⚠️ Number of userIds must equal number of taskIds',
    'problems.upload.modeHint': 'Please click the button under the textbox lol',
    'problems.upload.submitMultiple': 'Upload Problems',
    'problems.upload.error': 'Failed to upload problem',
    'problems.upload.errorInvalid': 'Please enter a valid URL',
    'problems.upload.errorInvalidData': 'Invalid data: number of userIds must equal number of taskIds',
    'problems.upload.errorNoUrls': 'Please enter at least one valid URL',
    'problems.upload.success': 'Problem uploaded successfully',
    'problems.upload.successMultiple': 'Bulk upload completed: {success} successful, {failed} failed',
    
    // Search
    'problems.search.title': 'Search Problems',
    'problems.search.placeholder': 'Enter search keyword...',
    'problems.search.submit': 'Search',
    'problems.search.searching': 'Searching...',
    'problems.search.error': 'Please enter a search keyword',
    'problems.search.errorFailed': 'Search failed',
    'problems.search.resultsFor': '【{keyword}】的搜索结果',
    'problems.search.found': '找到 {total} 条结果，显示 {displayed} 条',
    'problems.search.noResults': 'No results found',
    'problems.search.index': 'Index',
    'problems.search.problemTitle': 'Problem Title',
    'problems.search.time': 'Time',
    'problems.search.answer': 'Answer',
    'problems.search.ratio': 'Ratio',
    'problems.search.hot1': 'Hot1',
    'problems.search.hot1_full': 'The first choice of the hot comment',
    'problems.search.comment': 'Comment',
    'problems.search.comment_full': 'The overall choice of the comment section',
    'problems.search.detail': 'Detail',
    'problems.search.view': 'View',
    'problems.search.customizeColumns': 'Customize Columns',
    'problems.search.customizeTitle': 'Customize Table Columns',
    'problems.search.customizeDescription': 'Drag to reorder, toggle to show/hide columns',
    'problems.search.customizeClose': 'Close',
    'problems.search.customizeReset': 'Reset to Default',
    'problems.search.reorderMode': 'Reorder mode',
    'problems.search.modeDrag': 'Drag',
    'problems.search.modeClick': 'Click ↑↓',
    'problems.search.moveUp': 'Move up',
    'problems.search.moveDown': 'Move down',
    'problems.search.counts': 'Currently there are {elasticsearch} problems, with {redis} problems in process. Raw data: {mongodb} problems.',
    'problems.search.resultLimit': 'Results per page',
    'problems.search.resultLimitDesc': 'Adjust number of results (5-20)',
    
    // User page
    'user.title': 'MTV2',
    'user.problems': 'Problems',
    'user.welcome': 'Welcome, {username}',
    'user.changePassword': 'Change Password',
    'user.pointsInvitations': 'Points & Invitations',
    'user.oldPassword': 'Old Password',
    'user.newPassword': 'New Password',
    'user.confirmNewPassword': 'Confirm New Password',
    'user.submitPassword': 'Change Password',
    'user.submittingPassword': 'Changing...',
    'user.passwordError': 'Failed to change password',
    'user.passwordMismatch': 'New passwords do not match',
    'user.passwordTooShort': 'Password must be at least 6 characters',
    'user.passwordSuccess': 'Password changed successfully',
    'user.yourPoints': 'Your Points',
    'user.generateInvite': 'Generate Invitation Code',
    'user.inviteCount': 'Number of Codes (1-10)',
    'user.generateCodes': 'Generate Codes',
    'user.generating': 'Generating...',
    'user.inviteError': 'Failed to generate invitation codes',
    'user.inviteCountError': 'Count must be between 1 and 10',
    'user.inviteSuccess': 'Generated {count} invitation code(s) successfully',
    'user.myInvitations': 'My Invitation Codes',
    'user.loadingInvitations': 'Loading...',
    'user.noInvitations': 'No invitation codes generated yet',
    'user.created': 'Created',
    'user.used': 'Used',
    'user.available': 'Available',

    // User Stats page
    'userStats.title': 'User Detail',
    'userStats.totalLikes': '{count} Likes',
    'userStats.totalReplies': '{count} Replies',
    'userStats.comments': 'Comments ({count})',
    'userStats.supportUser': 'Support User',
    'userStats.supportMerchant': 'Support Merchant',
    'userStats.loading': 'Loading...',
    'userStats.notFound': 'User not found',
    'userStats.noComments': 'No comments yet',

    // Rankings page
    'rankings.title': 'Rankings',
    'pageTitle.rankings': 'Rankings',
    'rankings.rank': 'Rank',
    'rankings.user': 'User',
    'rankings.likes': 'Likes',
    'rankings.comments': 'Comments',
    'rankings.loading': 'Loading...',
    'rankings.empty': 'No rankings available',

    // Problem Operations
    'problemOps.title': 'Problem Operations',
    'problemOps.copyLink': 'Copy Link',
    'problemOps.copyJson': 'Copy JSON',
    'problemOps.copyYaml': 'Copy YAML',
    'problemOps.copySuccess': 'Copied to clipboard!',
    'problemOps.copyFailed': 'Failed to copy',
  },
  zh: {
    // Common
    'welcome': '欢迎',
    'loading': '加载中...',
    'logout': '退出登录',
    'language': '语言',
    'english': 'English',
    'chinese': '中文',
    'nav.home': '首页',
    'nav.problems': '题库',
    'nav.userStats': '查成分',
    
    // Page titles
    'pageTitle.home': '乐一欧应援网站',
    'pageTitle.login': '登录',
    'pageTitle.register': '注册',
    'pageTitle.problems': '题库',
    'pageTitle.user': '用户中心',
    
    // Home page
    'home.title': '掉心心了',
    'home.subtitle': '选择与结果不一致，掉小心心了！',
    
    // Login page
    'login.title': '欢迎回来',
    'login.subtitle': '登录您的账户',
    'login.username': '用户名',
    'login.usernamePlaceholder': '请输入您的用户名',
    'login.password': '密码',
    'login.passwordPlaceholder': '请输入您的密码',
    'login.submit': '登录',
    'login.submitting': '正在登录...',
    'login.error': '登录失败，请重试。',
    'login.noAccount': '还没有账户？',
    'login.signUp': '注册',
    'login.success': '账户创建成功！请登录。',
    'login.expired': '登录状态失效，请重新登录',
    
    // Register page
    'register.title': '创建账户',
    'register.subtitle': '使用邀请码注册',
    'register.inviteCode': '邀请码',
    'register.inviteCodePlaceholder': '请输入您的邀请码',
    'register.username': '用户名',
    'register.usernamePlaceholder': '选择一个用户名',
    'register.email': '邮箱',
    'register.emailPlaceholder': '请输入您的邮箱',
    'register.password': '密码',
    'register.passwordPlaceholder': '请输入密码（至少6个字符）',
    'register.confirmPassword': '确认密码',
    'register.confirmPasswordPlaceholder': '请确认您的密码',
    'register.submit': '创建账户',
    'register.submitting': '正在创建账户...',
    'register.error': '注册失败，请重试。',
    'register.passwordMismatch': '密码不匹配',
    'register.passwordTooShort': '密码至少需要6个字符',
    'register.hasAccount': '已有账户？',
    'register.signIn': '登录',
    'register.notice': '网站仅限大模型开发者使用，不支持对真实大众评审的滥用（如直接套用AI观点评审等），注册需要邀请码。',
    
    // Problems page
    'problems.title': '题库',
    'problems.userCenter': '用户中心',
    'problems.welcome': '欢迎, {username}',
    'problems.upload.title': '上传问题',
    'problems.upload.single': '单个上传',
    'problems.upload.multiple': '批量上传',
    'problems.upload.url': 'URL',
    'problems.upload.urlPlaceholder': '题目URL（点击右下角分享-复制链接）',
    'problems.upload.userId': '用户ID',
    'problems.upload.taskId': '任务ID',
    'problems.upload.submit': '上传问题',
    'problems.upload.submitting': '上传中...',
    'problems.upload.multipleUrls': 'URLs',
    'problems.upload.multipleUrlsPlaceholder': '教程：复制链接以后点一下评论的文本框，确保输入法收到复制，题目做完后依次点击输入法剪贴板内的各个复制项，然后上传...',
    'problems.upload.userIdsFound': '找到的用户ID',
    'problems.upload.taskIdsFound': '找到的任务ID',
    'problems.upload.validPairs': '有效配对（去重后）',
    'problems.upload.warning': '⚠️ 用户ID数量必须等于任务ID数量',
    'problems.upload.modeHint': '传题是下面的按钮哦',
    'problems.upload.submitMultiple': '上传问题',
    'problems.upload.error': '上传问题失败',
    'problems.upload.errorInvalid': '请输入有效的URL',
    'problems.upload.errorInvalidData': '数据无效：用户ID数量必须等于任务ID数量',
    'problems.upload.errorNoUrls': '请至少输入一个有效的URL',
    'problems.upload.success': '问题上传成功',
    'problems.upload.successMultiple': '批量上传完成：成功 {success} 个，失败 {failed} 个',
    
    // Search
    'problems.search.title': '搜索问题',
    'problems.search.placeholder': '输入搜索关键词...',
    'problems.search.submit': '搜索',
    'problems.search.searching': '搜索中...',
    'problems.search.error': '请输入搜索关键词',
    'problems.search.errorFailed': '搜索失败',
    'problems.search.resultsFor': '【{keyword}】的搜索结果',
    'problems.search.found': '找到 {total} 条结果，显示 {displayed} 条',
    'problems.search.noResults': '未找到结果',
    'problems.search.index': '序号',
    'problems.search.problemTitle': '问题标题',
    'problems.search.time': '时间',
    'problems.search.answer': '答案',
    'problems.search.ratio': '比例',
    'problems.search.hot1': '热评',
    'problems.search.hot1_full': '热评第一的选择',
    'problems.search.comment': '评论区',
    'problems.search.comment_full': '评论区总体的选择',
    'problems.search.detail': '详情',
    'problems.search.view': '查看',
    'problems.search.customizeColumns': '自定义列',
    'problems.search.customizeTitle': '自定义表格列',
    'problems.search.customizeDescription': '拖拽排序，切换显示/隐藏列',
    'problems.search.customizeClose': '关闭',
    'problems.search.customizeReset': '重置为默认',
    'problems.search.reorderMode': '排序模式',
    'problems.search.modeDrag': '拖拽',
    'problems.search.modeClick': '点击 ↑↓',
    'problems.search.moveUp': '上移',
    'problems.search.moveDown': '下移',
    'problems.search.counts': '当前共有 {elasticsearch} 题，上传队列中有 {redis} 题，原始数据 {mongodb} 题',
    'problems.search.resultLimit': '每页结果数',
    'problems.search.resultLimitDesc': '调整结果数量 (5-20)',
    
    // User page
    'user.title': 'MTV2',
    'user.problems': '问题',
    'user.welcome': '欢迎, {username}',
    'user.changePassword': '修改密码',
    'user.pointsInvitations': '积分与邀请',
    'user.oldPassword': '旧密码',
    'user.newPassword': '新密码',
    'user.confirmNewPassword': '确认新密码',
    'user.submitPassword': '修改密码',
    'user.submittingPassword': '修改中...',
    'user.passwordError': '修改密码失败',
    'user.passwordMismatch': '新密码不匹配',
    'user.passwordTooShort': '密码至少需要6个字符',
    'user.passwordSuccess': '密码修改成功',
    'user.yourPoints': '您的积分',
    'user.generateInvite': '生成邀请码',
    'user.inviteCount': '代码数量 (1-10)',
    'user.generateCodes': '生成代码',
    'user.generating': '生成中...',
    'user.inviteError': '生成邀请码失败',
    'user.inviteCountError': '数量必须在1到10之间',
    'user.inviteSuccess': '成功生成 {count} 个邀请码',
    'user.myInvitations': '我的邀请码',
    'user.loadingInvitations': '加载中...',
    'user.noInvitations': '尚未生成邀请码',
    'user.created': '创建时间',
    'user.used': '已使用',
    'user.available': '可用',

    // User Stats page
    'userStats.title': '查成分',
    'userStats.totalLikes': '共获得 {count} 个赞',
    'userStats.totalReplies': '共获得 {count} 次回复',
    'userStats.comments': '历史评论 ({count})',
    'userStats.supportUser': '适合展示',
    'userStats.supportMerchant': '不适合展示',
    'userStats.loading': '加载中...',
    'userStats.notFound': '用户不存在',
    'userStats.noComments': '暂无评论',

    // Rankings page
    'rankings.title': '排行榜',
    'pageTitle.rankings': '评论排行榜',
    'rankings.rank': '排名',
    'rankings.user': '用户',
    'rankings.likes': '获赞',
    'rankings.comments': '评论数',
    'rankings.loading': '加载中...',
    'rankings.empty': '暂无排行数据',

    // Problem Operations
    'problemOps.title': '题目操作',
    'problemOps.copyLink': '复制链接',
    'problemOps.copyJson': '复制JSON',
    'problemOps.copyYaml': '复制YAML',
    'problemOps.copySuccess': '已复制到剪贴板！',
    'problemOps.copyFailed': '复制失败',
  },
};

// Helper function to replace placeholders
function replacePlaceholders(text: string, params: Record<string, string | number>): string {
  return text.replace(/\{(\w+)\}/g, (match, key) => {
    const value = params[key];
    if (value === undefined || value === null) {
      return match;
    }
    return value.toString();
  });
}

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [language, setLanguageState] = useState<Language>('en');
  const [isInitialized, setIsInitialized] = useState(false);

  // Initialize language from localStorage or browser
  useEffect(() => {
    if (typeof window !== 'undefined' && !isInitialized) {
      const savedLanguage = localStorage.getItem('language') as Language | null;
      
      if (savedLanguage && (savedLanguage === 'en' || savedLanguage === 'zh')) {
        setLanguageState(savedLanguage);
      } else {
        // Detect browser language
        const browserLang = navigator.language || (navigator as any).userLanguage;
        const detectedLang = browserLang.startsWith('zh') ? 'zh' : 'en';
        setLanguageState(detectedLang);
        localStorage.setItem('language', detectedLang);
      }
      
      setIsInitialized(true);
    }
  }, [isInitialized]);

  const setLanguage = (lang: Language) => {
    setLanguageState(lang);
    if (typeof window !== 'undefined') {
      localStorage.setItem('language', lang);
    }
  };

  const t = (key: string, params?: Record<string, string | number>): string => {
    const translation = translations[language]?.[key] || key;
    if (params) {
      return replacePlaceholders(translation, params);
    }
    return translation;
  };

  return (
    <LanguageContext.Provider
      value={{
        language,
        setLanguage,
        t,
      }}
    >
      {children}
    </LanguageContext.Provider>
  );
}

export function useLanguage() {
  const context = useContext(LanguageContext);
  if (context === undefined) {
    throw new Error('useLanguage must be used within a LanguageProvider');
  }
  return context;
}
