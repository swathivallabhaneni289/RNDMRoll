import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { addComment, deleteComment, getPost, socialErrorMessage, type Comment, type Post } from '@/lib/api/board';
import { useForegroundReload } from '@/lib/api/useForegroundReload';
import { countAfterComment, placeComment, withoutComment } from '@/lib/board/comments';
import { isEntryId } from '@/lib/entry/rules';

export type PostLoad = {
  status: 'loading' | 'ready' | 'offline' | 'gone';
  failure: 'network' | 'generic';
  post: Post | null;
  reload: () => void;
  /** True once the post on screen is over 20 minutes old (its photo links may run out). */
  stale: () => boolean;
  /**
   * Sends a comment once, or a reply to `replyTo` while one is chosen; on success a comment is
   * added at the end and a reply after the last reply of its top-level comment, and reply mode
   * ends. Returns the failure line or null.
   */
  /** Sends a comment (a reply in reply mode); `photoKey` is an uploaded photo, and with it the text may be empty. */
  comment: (text: string, photoKey?: string | null) => Promise<string | null>;
  /** Deletes a comment (and, for a top-level one, its replies); 'not found' counts as done. */
  remove: (commentId: string) => Promise<string | null>;
  /**
   * Takes a comment you reported (and, for a top-level one, its replies) off the post on screen,
   * with no request: it is gone for you from then on.
   */
  drop: (commentId: string) => void;
  /** The comment the bar is replying to, or null for a plain comment. */
  replyTo: Comment | null;
  /** Puts the bar into reply mode for `comment` (each tap is a new choice, so the bar refills). */
  startReply: (comment: Comment) => void;
  cancelReply: () => void;
};

/**
 * One post (GET /posts/{entry_id}) for the post page and for the extras of your own entry page.
 * Reads again on focus, and on a return from the background once 20 minutes old (its photo links
 * stop working within an hour); what is on screen stays while a later read runs or fails. A 404
 * (deleted, a block, never visible) or an id that is not a UUID is `gone`. A ref keeps each action
 * to one request at a time.
 */
export function usePost(id: string): PostLoad {
  const valid = isEntryId(id);
  const [post, setPost] = useState<Post | null>(null);
  const [status, setStatus] = useState<PostLoad['status']>(valid ? 'loading' : 'gone');
  const [failure, setFailure] = useState<PostLoad['failure']>('network');

  const postRef = useRef<Post | null>(null);
  const seqRef = useRef(0);
  const mountedRef = useRef(true);
  const commentingRef = useRef(false);
  const [replyTo, setReplyTo] = useState<Comment | null>(null);
  const replyToRef = useRef<Comment | null>(null);

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

  const { markLoaded, stale } = useForegroundReload(() => void load());

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
      markLoaded();
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
  }, [id, valid, put, markLoaded]);

  useFocusEffect(
    useCallback(() => {
      void load();
    }, [load])
  );

  const chooseReply = useCallback((next: Comment | null) => {
    replyToRef.current = next;
    setReplyTo(next);
  }, []);

  const startReply = useCallback((target: Comment) => chooseReply({ ...target }), [chooseReply]);
  const cancelReply = useCallback(() => chooseReply(null), [chooseReply]);

  const comment = useCallback(
    async (text: string, photoKey: string | null = null): Promise<string | null> => {
      const body = text.trim();
      if ((body === '' && photoKey === null) || commentingRef.current || postRef.current === null) return null;
      commentingRef.current = true;
      const parent = replyToRef.current;
      try {
        const saved = await addComment(id, body, parent?.id ?? null, photoKey);
        const now = postRef.current;
        if (mountedRef.current && now !== null) {
          const list = now.comments ?? [];
          put({
            ...now,
            comments: placeComment(list, saved),
            card: { ...now.card, comment_count: countAfterComment(list, now.card.comment_count, saved) },
          });
        }
        if (mountedRef.current && replyToRef.current === parent) chooseReply(null);
        return null;
      } catch (err) {
        if (err instanceof ApiError && err.code === 'comments_off') void load();
        if (err instanceof ApiError && err.code === 'not_found') {
          // The post went away (deleted, archived, a block, no longer followed), or for a reply maybe
          // only the comment being answered: reply mode ends, and the post is read again, which
          // turns a gone post into its 'not here' line instead of a Post that fails on every tap.
          if (parent !== null && mountedRef.current && replyToRef.current === parent) chooseReply(null);
          void load();
        }
        return socialErrorMessage(err, parent !== null ? 'reply' : 'comment');
      } finally {
        commentingRef.current = false;
      }
    },
    [id, put, load, chooseReply]
  );

  // A top-level comment goes with its replies, the count drops by all that left, and reply mode
  // ends when the comment being replied to left (after a delete, and after a report).
  const drop = useCallback(
    (commentId: string) => {
      const now = postRef.current;
      if (!mountedRef.current || now === null) return;
      const after = withoutComment(now.comments ?? [], now.card.comment_count, commentId, replyToRef.current?.id ?? null);
      if (after.endReply) chooseReply(null);
      put({ ...now, comments: after.list, card: { ...now.card, comment_count: after.count } });
    },
    [put, chooseReply]
  );

  const remove = useCallback(
    async (commentId: string): Promise<string | null> => {
      try {
        await deleteComment(commentId);
        drop(commentId);
        return null;
      } catch (err) {
        return socialErrorMessage(err, 'other');
      }
    },
    [drop]
  );

  return { status, failure, post, reload: () => void load(), stale, comment, remove, drop, replyTo, startReply, cancelReply };
}
