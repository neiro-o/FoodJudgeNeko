export type RegularUpload = { userId: string; taskId: string };
export type DailyUpload = { userId: string; dateId: string };

const URL_START_PATTERN = /https?:\/\//gi;
const URL_SEPARATOR_PATTERN = /[\s,，;；]/;

/**
 * Extract pasted URLs even when adjacent URLs have no separator. Protocol
 * markers are the authoritative boundaries; whitespace and punctuation only
 * trim the tail of each segment.
 */
export const extractUploadUrls = (text: string): string[] => {
  const starts = Array.from(text.matchAll(URL_START_PATTERN), (match) => match.index ?? -1)
    .filter((index) => index >= 0);

  return starts.flatMap((start, index) => {
    const end = starts[index + 1] ?? text.length;
    const segment = text.slice(start, end);
    const separatorIndex = segment.search(URL_SEPARATOR_PATTERN);
    const url = (separatorIndex >= 0 ? segment.slice(0, separatorIndex) : segment).trim();
    return url ? [url] : [];
  });
};

/** Keep exactly the first Base64 padding marker and discard pasted suffixes. */
export const normalizeTaskId = (taskId: string | null): string | null => {
  if (!taskId) return null;
  const paddingIndex = taskId.indexOf('==');
  return paddingIndex >= 0 ? taskId.slice(0, paddingIndex + 2) : null;
};

export const parseProblemUrl = (url: string): { userId: string | null; taskId: string | null } => {
  try {
    const urlObject = new URL(url);
    return {
      userId: urlObject.searchParams.get('userId'),
      taskId: normalizeTaskId(urlObject.searchParams.get('encryptMockTaskNo')),
    };
  } catch {
    return { userId: null, taskId: null };
  }
};

export const parseDailyUrl = (url: string): { userId: string | null; dateId: string | null } => {
  try {
    const urlObject = new URL(url);
    if (urlObject.searchParams.get('jumpScene') !== 'dailyReport') {
      return { userId: null, dateId: null };
    }
    return {
      userId: urlObject.searchParams.get('shareUserId'),
      dateId: urlObject.searchParams.get('dailyReportTime'),
    };
  } catch {
    return { userId: null, dateId: null };
  }
};

export const analyzeUnifiedUploads = (text: string) => {
  const urls = extractUploadUrls(text);
  const regular: RegularUpload[] = [];
  const daily: DailyUpload[] = [];
  let invalid = 0;

  for (const url of urls) {
    const problem = parseProblemUrl(url);
    if (problem.userId && problem.taskId) {
      regular.push({ userId: problem.userId, taskId: problem.taskId });
      continue;
    }

    const report = parseDailyUrl(url);
    if (report.userId && report.dateId) {
      daily.push({ userId: report.userId, dateId: report.dateId });
      continue;
    }
    invalid++;
  }

  const uniqueRegular = regular.filter((item, index, items) =>
    index === items.findIndex((candidate) => candidate.userId === item.userId && candidate.taskId === item.taskId)
  );
  const uniqueDaily = daily.filter((item, index, items) =>
    index === items.findIndex((candidate) => candidate.userId === item.userId && candidate.dateId === item.dateId)
  );

  return {
    regular: uniqueRegular,
    daily: uniqueDaily,
    total: uniqueRegular.length + uniqueDaily.length,
    invalid,
  };
};
