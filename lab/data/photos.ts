import { useMemo, useSyncExternalStore } from 'react';
import type { LabEntry, SceneId } from '@/lab/data/entries';

/**
 * A photo is either one of the designed placeholder scenes or a URI the developer
 * picked from their own library. Real, user-captured photos are the only photo
 * source this product allows (no stock, no generated imagery), so the lab never
 * bundles photographs: it ships abstract monochrome scenes and lets the developer
 * swap in their own.
 */
export type PhotoRef = { kind: 'scene'; id: SceneId } | { kind: 'uri'; uri: string };

type Listener = () => void;

let userPhotos: readonly string[] = [];
const listeners = new Set<Listener>();

function subscribe(listener: Listener) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot() {
  return userPhotos;
}

export function setUserPhotos(uris: readonly string[]) {
  userPhotos = uris;
  listeners.forEach((listener) => listener());
}

export function useUserPhotos(): readonly string[] {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

/** The photo an entry shows right now: the developer's own if any were picked. */
export function usePhoto(entry: LabEntry): PhotoRef {
  const photos = useUserPhotos();
  return useMemo<PhotoRef>(
    () => (photos.length > 0 ? { kind: 'uri', uri: photos[entry.slot % photos.length] } : { kind: 'scene', id: entry.scene }),
    [photos, entry.slot, entry.scene],
  );
}
