import { useCallback, useEffect, useRef, useState } from 'react';
import type { Entry, PhotoSource } from '@/lib/api/entries';
import { createUploadMemory, canAddPhoto, photoListChanged, type UploadMemory } from '@/lib/entry/photo';
import type { ReactionName } from '@/lib/entry/reactions';
import { missingParts, type MissingPart } from '@/lib/entry/rules';
import { STARS_MAX } from '@/lib/entry/stars';

/**
 * One photo of the draft, in order; the first is the cover. A picked file has a local `uri` and no
 * `id`. In edit mode an existing photo is the entry's own photo: `id` is its server id and `uri` the
 * link it came with, only for showing it (the server signs a new link for the same photo every half
 * hour, so photos are told apart by `id`); `mimeType` is null and `source` is the entry's
 * `photo_source`.
 */
export type PickedPhoto = { uri: string; mimeType: string | null; source: PhotoSource; id?: string };

type Values = {
  photos: PickedPhoto[];
  title: string;
  stars: number;
  reaction: ReactionName | null;
  comment: string;
};

const EMPTY: Values = { photos: [], title: '', stars: 0, reaction: null, comment: '' };

function valuesFrom(entry: Entry | null | undefined): Values {
  if (!entry) return EMPTY;
  const ordered = [...entry.photos].sort((a, b) => a.position - b.position);
  return {
    photos: ordered.map((photo) => ({ uri: photo.url, mimeType: null, source: entry.photo_source, id: photo.id })),
    title: entry.title,
    stars: entry.stars,
    reaction: entry.reaction,
    comment: entry.comment ?? '',
  };
}

export type EntryDraft = {
  photos: PickedPhoto[];
  title: string;
  stars: number;
  reaction: ReactionName | null;
  comment: string;
  setTitle: (text: string) => void;
  setStars: (value: number) => void;
  setReaction: (next: ReactionName | null) => void;
  setComment: (text: string) => void;
  /** Adds a photo at the end. Ignored at 10 photos and for a `uri` already in the list. */
  addPhoto: (photo: PickedPhoto) => void;
  /** Removes the photo with this `uri` and drops its remembered upload key. */
  removePhoto: (uri: string) => void;
  /** Anything differs from where the draft started (empty for a new entry, the entry for edit). */
  dirty: boolean;
  /** What stops Save, in the order photo, title, stars. */
  missing: MissingPart[];
  /** Back to where the draft started; forgets every upload key and clears `saved`. */
  reset: () => void;
  saved: boolean;
  /** Call right before leaving after a successful save: the leave guard then stays quiet. */
  markSaved: () => void;
  /** `dirty` and not `saved`, for `gestureEnabled`. */
  unsaved: boolean;
  /** The same, read from refs at the moment of the call: use it inside a `beforeRemove` listener. */
  isUnsaved: () => boolean;
  /** The one upload memory: `ensure(uri, upload)` uploads a picked file once, a retry reuses its key. */
  memory: UploadMemory;
};

/**
 * The entry form's state (plan 03-02 Task 2 f). Starts empty, or from an existing entry for edit
 * mode (plan 03-03): it starts once, and again only if a different entry id arrives, so a reload
 * of the same entry never overwrites what the person typed.
 */
export function useEntryDraft(initial?: Entry | null): EntryDraft {
  const [baseline, setBaseline] = useState<Values>(() => valuesFrom(initial));
  const [photos, setPhotos] = useState<PickedPhoto[]>(baseline.photos);
  const [title, setTitle] = useState(baseline.title);
  const [stars, setStarsState] = useState(baseline.stars);
  const [reaction, setReaction] = useState<ReactionName | null>(baseline.reaction);
  const [comment, setComment] = useState(baseline.comment);
  const [saved, setSaved] = useState(false);
  const seededFor = useRef<string | null>(initial?.id ?? null);
  const savedRef = useRef(false);
  const dirtyRef = useRef(false);

  const memoryRef = useRef<UploadMemory | null>(null);
  if (memoryRef.current === null) memoryRef.current = createUploadMemory();
  const memory = memoryRef.current;

  useEffect(() => {
    if (!initial || seededFor.current === initial.id) return;
    seededFor.current = initial.id;
    const next = valuesFrom(initial);
    memory.forget();
    setBaseline(next);
    setPhotos(next.photos);
    setTitle(next.title);
    setStarsState(next.stars);
    setReaction(next.reaction);
    setComment(next.comment);
  }, [initial, memory]);

  const addPhoto = useCallback((photo: PickedPhoto) => {
    setPhotos((current) => {
      if (!canAddPhoto(current.length)) return current;
      if (current.some((item) => item.uri === photo.uri)) return current;
      return [...current, photo];
    });
  }, []);

  const removePhoto = useCallback(
    (uri: string) => {
      memory.forget(uri);
      setPhotos((current) => current.filter((item) => item.uri !== uri));
    },
    [memory]
  );

  const setStars = useCallback((value: number) => {
    const whole = Number.isFinite(value) ? Math.trunc(value) : 0;
    setStarsState(Math.min(STARS_MAX, Math.max(0, whole)));
  }, []);

  const reset = useCallback(() => {
    memory.forget();
    savedRef.current = false;
    setSaved(false);
    setPhotos(baseline.photos);
    setTitle(baseline.title);
    setStarsState(baseline.stars);
    setReaction(baseline.reaction);
    setComment(baseline.comment);
  }, [baseline, memory]);

  const markSaved = useCallback(() => {
    savedRef.current = true;
    setSaved(true);
  }, []);

  const dirty =
    photoListChanged(photos, baseline.photos) ||
    title.trim() !== baseline.title.trim() ||
    stars !== baseline.stars ||
    reaction !== baseline.reaction ||
    comment.trim() !== baseline.comment.trim();
  dirtyRef.current = dirty;

  const isUnsaved = useCallback(() => dirtyRef.current && !savedRef.current, []);

  return {
    photos,
    title,
    stars,
    reaction,
    comment,
    setTitle,
    setStars,
    setReaction,
    setComment,
    addPhoto,
    removePhoto,
    dirty,
    missing: missingParts({ hasPhoto: photos.length > 0, title, stars }),
    reset,
    saved,
    markSaved,
    unsaved: dirty && !saved,
    isUnsaved,
    memory,
  };
}
