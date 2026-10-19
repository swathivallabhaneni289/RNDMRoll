/**
 * Links from outside the app. An invite link's button (rndmroll://u/{username}, the page at
 * /i/{username}) becomes that person's profile on the Board tab, so Back goes to the Board as from
 * any profile. Every other link is left as it came.
 */
export function redirectSystemPath({ path }: { path: string; initial: boolean }): string {
  const match = /^(?:[a-z]+:\/\/)?\/?u\/([A-Za-z0-9_]{3,20})\/?$/.exec(path);
  return match ? `/board/person?username=${match[1].toLowerCase()}` : path;
}
