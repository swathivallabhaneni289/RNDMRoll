import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigation, useRouter } from 'expo-router';
import type { EntryDraft } from '@/components/entry/useEntryDraft';
import { ApiError } from '@/lib/api/client';
import { createEntry, entryErrorMessage, uploadEntryPhoto, type CreateEntryRequest } from '@/lib/api/entries';
import { cleanTitle } from '@/lib/entry/rules';
import { applyToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/** The spin the entry screen kept when it opened: the only two things the post names. */
export type ShownSpin = { localDate: string; seq: number };

export type CreateEntryState = {
  /** One save. A second call while one runs, after success, or after a terminal answer does nothing. */
  submit: () => Promise<void>;
  submitting: boolean;
  /** The failure line above the Save button (UI spec 3.6). */
  message: string | null;
  /** A refused or failed photo, shown under the photo strip (UI spec 7). */
  photoMessage: string | null;
  /** The day moved on: the button reads 'Back to Today' and `backToToday` leaves. */
  terminal: boolean;
  backToToday: () => void;
};

/**
 * One save of the entry screen (plan 03-02 Task 3b, UI spec 4 'Saving').
 *
 * Each photo is uploaded once, one after another, through the draft's upload memory, so a retry
 * names the keys it already has and never uploads a file twice. Then the post names the spin the
 * screen showed (`for_date` and `spin_seq`); the phone never works out the date, the window or
 * whether the post is late. The answer carries the new Today, which goes into the store.
 *
 * Failures: a refused or failed photo puts its message under the strip and posts nothing; a lost
 * connection and every other failure show on the line above Save with the form as it was; the four
 * state answers end the attempt (the button becomes 'Back to Today') and Today is asked for again.
 */
export function useCreateEntry({
  userId,
  spin,
  draft,
}: {
  userId: string | null;
  spin: ShownSpin | null;
  draft: EntryDraft;
}): CreateEntryState {
  const router = useRouter();
  const navigation = useNavigation();
  const [submitting, setSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [photoMessage, setPhotoMessage] = useState<string | null>(null);
  const [terminal, setTerminal] = useState(false);

  /** One save at a time, and none after one has succeeded (the screen is on its way out). */
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
    // The attempt cannot go on, so nothing is left to protect: the leave guard stays quiet.
    draft.markSaved();
    goBack();
  }, [draft, goBack]);

  const submit = useCallback(async () => {
    if (busyRef.current || doneRef.current || endedRef.current) return;
    if (userId === null || spin === null || draft.missing.length > 0) return;

    busyRef.current = true;
    setSubmitting(true);
    setMessage(null);
    setPhotoMessage(null);

    let succeeded = false;
    try {
      // One photo after another. A key already returned for this file is reused, so a retry
      // never uploads it again; a failed upload is not remembered and is tried again.
      const uploaded: CreateEntryRequest['photos'] = [];
      try {
        for (const photo of draft.photos) {
          const key = await draft.memory.ensure(photo.uri, () =>
            uploadEntryPhoto(photo.uri, photo.mimeType ?? 'image/jpeg')
          );
          uploaded.push({ photo_key: key, photo_source: photo.source });
        }
      } catch (err) {
        if (err instanceof ApiError && (err.code === 'photo_rejected' || err.code === 'photo_upload_failed')) {
          // The photo is the problem: its message goes under the strip and nothing is posted.
          if (mountedRef.current) setPhotoMessage(entryErrorMessage(err).text);
          return;
        }
        throw err;
      }

      const body: CreateEntryRequest = {
        for_date: spin.localDate,
        spin_seq: spin.seq,
        title: cleanTitle(draft.title),
        stars: draft.stars,
        photos: uploaded,
      };
      if (draft.reaction !== null) body.reaction = draft.reaction;
      const comment = draft.comment.trim();
      if (comment.length > 0) body.comment = comment;

      // The countdown anchor counts from the moment this request starts, not from the uploads.
      const mark = performance.now();
      const answer = await createEntry(body);

      // The new Today goes in first. A Today asked for before this post can still answer after
      // it, so Today is asked for again afterwards: that answer is the newest.
      applyToday(userId, answer.today, mark);
      void refetchToday(userId, 'manual');

      succeeded = true;
      doneRef.current = true;
      draft.markSaved();
      // If the person already left (Cancel, then Leave, while this ran) the screen is gone:
      // going back again would close the wrong one.
      if (mountedRef.current && navigation.isFocused()) goBack();
    } catch (err) {
      const failure = entryErrorMessage(err);
      if (mountedRef.current) setMessage(failure.text);
      if (failure.terminal) {
        endedRef.current = true;
        if (mountedRef.current) setTerminal(true);
        // The day moved on: Today asks the server again so it shows the truth.
        void refetchToday(userId, 'manual');
      }
    } finally {
      busyRef.current = false;
      // After a success the button stays inert until the screen has gone.
      if (!succeeded && mountedRef.current) setSubmitting(false);
    }
  }, [userId, spin, draft, navigation, goBack]);

  return { submit, submitting, message, photoMessage, terminal, backToToday };
}
