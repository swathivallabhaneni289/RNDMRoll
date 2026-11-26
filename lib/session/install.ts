/**
 * Whether the login kept in the keychain belongs to this install of the app. iOS keeps keychain
 * items after the app is deleted, so without this a fresh install would open on the last login.
 * The app keeps an install id in AsyncStorage, which iOS deletes with the app, and the same id in
 * the keychain beside the login. lib/session/store.ts reads and writes both at launch, before the
 * login is read. Pure: no imports, so scripts/check-accounts-extra.ts runs it.
 */

/**
 * What a launch does with the keychain login:
 *  - 'keep': the two ids match. Nothing is written.
 *  - 'stamp': the keychain has no id yet, so the login (if any) is from before install ids, or the
 *    stamp was cut short; or it has an id but no login to clear. The login stays, and both ids
 *    are written: a new one in AsyncStorage when it has none, and the same one in the keychain.
 *  - 'clear': the keychain holds a login and an id that is not this install's (AsyncStorage has
 *    none, or another one): the app was deleted and installed again. The login goes, then both
 *    ids are written as for 'stamp', and the app starts signed out.
 */
export type InstallStep = 'keep' | 'stamp' | 'clear';

/**
 * storedId: the id in AsyncStorage. keychainId: the id in the keychain. hasLogin: the keychain
 * holds a token. Null means read and not there: a read that failed decides nothing (the store
 * never calls this then), or an id missing for a moment would clear a login that is this
 * install's.
 */
export function installStep(storedId: string | null, keychainId: string | null, hasLogin: boolean): InstallStep {
  if (keychainId === null) return 'stamp';
  if (keychainId === storedId) return 'keep';
  return hasLogin ? 'clear' : 'stamp';
}
