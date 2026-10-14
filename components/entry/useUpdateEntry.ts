import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigation, useRouter } from 'expo-router';
import { goToToday } from '@/components/entry/goToToday';
import type { EntryDraft } from '@/components/entry/useEntryDraft';
import { ApiError } from '@/lib/api/client';
import {
  entryErrorMessage,
  updateEntry,
  uploadEntryPhoto,
  type Entry,
  type UpdateEntryRequest,
} from '@/lib/api/entries';
import { changedFields } from '@/lib/entry/rules';
import { refetchToday } from '@/lib/wheel/useToday';

export type UpdateEntryState = {
  /** One save. A second call while one runs, after success, or after a terminal answer does nothing. */
  submit: () => Promise<void>;
  submitting: boolean;
  /** The failure line above the Save button. */
  message: string | null;
  /** A refused or failed photo, shown under the photo strip. */
  photoMessage: string | null;
  /** The entry is gone: the button reads 'Back to Today' and `backToToday` leaves for Today. */
  terminal: boolean;
  backToToday: () => void;
};

/**
 * One save of edit entry (plan 03-03 Task 1d, UI spec 10). Only the fields the person changed
 * are sent. When the photos changed, the whole ordered list is sent: `{ id }` keeps a photo and
 * `{ photo_key, photo_source }` adds a new one, each new file uploaded once through the draft's
 * upload memory (a retry reuses its key). Clearing the reaction or the comment sends ''.
 *
 * A PATCH answers with the entry only, so Today is asked for again afterwards: its Posted row
 * would otherwise keep the old title, stars and cover.
 */
export function useUpdateEntry({
  userId,
  original,
  draft,
}: {
  userId: string | null;
  /** The entry the draft started from. */
  original: Entry;
  draft: EntryDraft;
}): UpdateEntryState {
  const router = useRouter();
  const navigation = useNavigation();
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [photoMessage, setPhotoMessage] = useState<string | null>(null);
  const [terminal, setTerminal] = useState(false);

  const busyRef = useRef(false);
  const doneRef = useRef(false);
  const endedRef = useRef(false);
  const mountedRef = useRef(true);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const goBack = useCallback(() => {
    if (router.canGoBack()) router.back();
    else router.replace('/(app)/today');
  }, [router]);

  const backToToday = useCallback(() => {
    // Nothing is left to protect: the leave guard stays quiet.
    draft.markSaved();
    goToToday(router);
  }, [draft, router]);

  const submit = useCallback(async () => {
    if (busyRef.current || doneRef.current || endedRef.current) return;
    if (draft.missing.length > 0) return;

    busyRef.current = true;
    setSubmitting(true);
    setMessage(null);
    setPhotoMessage(null);

    let succeeded = false;
    try {
      const patch: UpdateEntryRequest = {};
      const fields = changedFields(original, {
        title: draft.title,
        stars: draft.stars,
        reaction: draft.reaction,
        comment: draft.comment,
      });
      if (fields.title !== undefined) patch.title = fields.title;
      if (fields.stars !== undefined) patch.stars = fields.stars;
      if (fields.reaction !== undefined) patch.reaction = fields.reaction as UpdateEntryRequest['reaction'];
      if (fields.comment !== undefined) patch.comment = fields.comment;

      const originalUris = [...original.photos].sort((a, b) => a.position - b.position).map((photo) => photo.url);
      const photosChanged =
        draft.photos.length !== originalUris.length || draft.photos.some((photo, i) => photo.uri !== originalUris[i]);

      if (photosChanged) {
        const ordered: NonNullable<UpdateEntryRequest['photos']> = [];
        try {
          for (const photo of draft.photos) {
            const existing = draft.existingPhotoId(photo.uri);
            if (existing !== null) {
              ordered.push({ id: existing });
              continue;
            }
            // A new file: uploaded once; a retry reuses the key it already has.
            const key = await draft.memory.ensure(photo.uri, () =>
              uploadEntryPhoto(photo.uri, photo.mimeType ?? 'image/jpeg')
            );
            ordered.push({ photo_key: key, photo_source: photo.source });
          }
        } catch (err) {
          if (err instanceof ApiError && (err.code === 'photo_rejected' || err.code === 'photo_upload_failed')) {
            if (mountedRef.current) setPhotoMessage(entryErrorMessage(err).text);
            return;
          }
          throw err;
        }
        patch.photos = ordered;
      }

      if (Object.keys(patch).length > 0) {
        await updateEntry(original.id, patch);
        if (userId !== null) void refetchToday(userId, 'manual');
      }

      succeeded = true;
      doneRef.current = true;
      draft.markSaved();
      if (mountedRef.current && navigation.isFocused()) goBack();
    } catch (err) {
      const failure = entryErrorMessage(err);
      if (mountedRef.current) setMessage(failure.text);
      if (failure.terminal) {
        endedRef.current = true;
        if (mountedRef.current) setTerminal(true);
        if (userId !== null) void refetchToday(userId, 'manual');
      }
    } finally {
      busyRef.current = false;
      if (!succeeded && mountedRef.current) setSubmitting(false);
    }
  }, [userId, original, draft, navigation, goBack]);

  return { submit, submitting, message, photoMessage, terminal, backToToday };
}
