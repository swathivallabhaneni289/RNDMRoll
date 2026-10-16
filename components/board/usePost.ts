import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { addComment, deleteComment, getPost, setReaction, socialErrorMessage, type Post } from '@/lib/api/board';
import type { ReactionName } from '@/lib/entry/reactions';
import { isEntryId } from '@/lib/entry/rules';

export type PostLoad = {
  status: 'loading' | 'ready' | 'offline' | 'gone';
  failure: 'network' | 'generic';
  post: Post | null;
  reload: () => void;
  /** Sets your reaction at once and sends it; on a failure it goes back and the line is returned. */
  react: (next: ReactionName | null) => Promise<string | null>;
  /** Sends a comment once; on success it is added at the end. Returns the failure line or null. */
  comment: (text: string) => Promise<string | null>;
  /** Deletes a comment; 'not found' counts as done. Returns the failure line or null. */
  remove: (commentId: string) => Promise<string | null>;
};

/**
 * One post (GET /posts/{entry_id}) for the post page and for the extras of your own entry page.
 * Reads again on focus; what is on screen stays while a later read runs or fails. A 404 (deleted,
 * a block, never visible) or an id that is not a UUID is `gone`. A ref keeps each action to one
 * request at a time.
 */
export function usePost(id: string): PostLoad {
  const valid = isEntryId(id);
  const [post, setPost] = useState<Post | null>(null);
  const [status, setStatus] = useState<PostLoad['status']>(valid ? 'loading' : 'gone');
  const [failure, setFailure] = useState<PostLoad['failure']>('network');

  const postRef = useRef<Post | null>(null);
  const seqRef = useRef(0);
  const mountedRef = useRef(true);
  const reactingRef = useRef(false);
  const commentingRef = useRef(false);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const put = useCallback((next: Post | null) => {
    postRef.current = next;
    setPost(next);
  }, []);

  const load = useCallback(async () => {
    if (!valid) {
      put(null);
      setStatus('gone');
      return;
    }
    const seq = ++seqRef.current;
    if (postRef.current === null) setStatus('loading');
    try {
      const next = await getPost(id);
      if (!mountedRef.current || seq !== seqRef.current) return;
      put(next);
      setStatus('ready');
    } catch (err) {
      if (!mountedRef.current || seq !== seqRef.current) return;
      if (err instanceof ApiError && err.code === 'not_found') {
        put(null);
        setStatus('gone');
        return;
      }
      if (postRef.current !== null) return;
      setFailure(err instanceof ApiError && err.code === 'network_unavailable' ? 'network' : 'generic');
      setStatus('offline');
    }
  }, [id, valid, put]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  const react = useCallback(
    async (next: ReactionName | null): Promise<string | null> => {
      const before = postRef.current;
      if (before === null || reactingRef.current || before.card.mine || !before.can_react) return null;
      reactingRef.current = true;
      const was = before.card.my_reaction;
      put({ ...before, card: { ...before.card, my_reaction: next } });
      try {
        const saved = await setReaction(id, next);
        const now = postRef.current;
        if (mountedRef.current && now !== null) put({ ...now, card: { ...now.card, my_reaction: saved } });
        return null;
      } catch (err) {
        const now = postRef.current;
        if (mountedRef.current && now !== null) put({ ...now, card: { ...now.card, my_reaction: was } });
        return socialErrorMessage(err, 'reaction');
      } finally {
        reactingRef.current = false;
      }
    },
    [id, put]
  );

  const comment = useCallback(
    async (text: string): Promise<string | null> => {
      const body = text.trim();
      if (body === '' || commentingRef.current || postRef.current === null) return null;
      commentingRef.current = true;
      try {
        const saved = await addComment(id, body);
        const now = postRef.current;
        if (mountedRef.current && now !== null) {
          const list = now.comments ?? [];
          put({
            ...now,
            comments: [...list.filter((item) => item.id !== saved.id), saved],
            card: { ...now.card, comment_count: (now.card.comment_count ?? list.length) + 1 },
          });
        }
        return null;
      } catch (err) {
        if (err instanceof ApiError && err.code === 'comments_off') void load();
        return socialErrorMessage(err, 'comment');
      } finally {
        commentingRef.current = false;
      }
    },
    [id, put, load]
  );

  const remove = useCallback(
    async (commentId: string): Promise<string | null> => {
      try {
        await deleteComment(commentId);
        const now = postRef.current;
        if (mountedRef.current && now !== null) {
          const list = now.comments ?? [];
          const left = list.filter((item) => item.id !== commentId);
          put({
            ...now,
            comments: left,
            card: { ...now.card, comment_count: Math.max(0, (now.card.comment_count ?? list.length) - (list.length - left.length)) },
          });
        }
        return null;
      } catch (err) {
        return socialErrorMessage(err, 'other');
      }
    },
    [put]
  );

  return { status, failure, post, reload: () => void load(), react, comment, remove };
}
