// XOR + base64 obfuscation avoids storing the app URL template in plain text.
const appLinkTemplate = 'Oj42OicmMj1pfHwkJCR9PjY6JyYyPX0wPD58JDYxbCYhP24=';

export function buildProblemAppLink(link: string): string {
  const prefix = Array.from(atob(appLinkTemplate), char => String.fromCharCode(char.charCodeAt(0) ^ 0x53)).join('');
  return prefix + encodeURIComponent(link);
}
