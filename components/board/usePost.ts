import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { addComment, deleteComment, getPost, socialErrorMessage, type Comment, type Post } from '@/lib/api/board';
import { isEntryId } from '@/lib/entry/rules';

export type PostLoad = {
  status: 'loading' | 'ready' | 'offline' | 'gone';
  failure: 'network' | 'generic';
  post: Post | null;
  reload: () => void;
  /**
   * Sends a comment once, or a reply to `replyTo` while one is chosen; on success a comment is
   * added at the end and a reply after the last reply of its top-level comment, and reply mode
   * ends. Returns the failure line or null.
   */
  /** Sends a comment (a reply in reply mode); `photoKey` is an uploaded photo, and with it the text may be empty. */
  comment: (text: string, photoKey?: string | null) => Promise<string | null>;
  /** Deletes a comment (and, for a top-level one, its replies); 'not found' counts as done. */
  remove: (commentId: string) => Promise<string | null>;
  /** The comment the bar is replying to, or null for a plain comment. */
  replyTo: Comment | null;
  /** Puts the bar into reply mode for `comment` (each tap is a new choice, so the bar refills). */
  startReply: (comment: Comment) => void;
  cancelReply: () => void;
};

/** `list` with `saved` placed after the last comment of its thread (a reply), or at the end. */
function placeComment(list: Comment[], saved: Comment): Comment[] {
  const rest = list.filter((item) => item.id !== saved.id);
  const top = saved.parent_id;
  if (top === null) return [...rest, saved];
  let at = -1;
  rest.forEach((item, index) => {
    if (item.id === top || item.parent_id === top) at = index;
  });
  if (at === -1) return [...rest, saved];
  return [...rest.slice(0, at + 1), saved, ...rest.slice(at + 1)];
}

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
            card: { ...now.card, comment_count: (now.card.comment_count ?? list.length) + 1 },
          });
        }
        if (mountedRef.current && replyToRef.current === parent) chooseReply(null);
        return null;
      } catch (err) {
        if (err instanceof ApiError && err.code === 'comments_off') void load();
        if (parent !== null && err instanceof ApiError && err.code === 'not_found') {
          // The comment being answered went away: reply mode ends and the list is read again.
          if (mountedRef.current && replyToRef.current === parent) chooseReply(null);
          void load();
        }
        return socialErrorMessage(err, parent !== null ? 'reply' : 'comment');
      } finally {
        commentingRef.current = false;
      }
    },
    [id, put, load, chooseReply]
  );

  const remove = useCallback(
    async (commentId: string): Promise<string | null> => {
      try {
        await deleteComment(commentId);
        const now = postRef.current;
        if (mountedRef.current && now !== null) {
          const list = now.comments ?? [];
          // A top-level comment goes with its replies.
          const left = list.filter((item) => item.id !== commentId && item.parent_id !== commentId);
          const target = replyToRef.current;
          if (target !== null && !left.some((item) => item.id === target.id)) chooseReply(null);
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
    [put, chooseReply]
  );

  return { status, failure, post, reload: () => void load(), comment, remove, replyTo, startReply, cancelReply };
}
