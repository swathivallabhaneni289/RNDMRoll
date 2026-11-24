import { useCallback, useEffect, useRef, useState } from 'react';
import { useFocusEffect } from 'expo-router';
import { ApiError } from '@/lib/api/client';
import { addComment, deleteComment, getPost, setLike, socialErrorMessage, type Comment, type Post } from '@/lib/api/board';
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
   * with no request: it is gone for you from then on. A read already on its way is dropped, so it
   * can't bring the comment back; the next read leaves it out.
   */
  drop: (commentId: string) => void;
  /** The comment the bar is replying to, or null for a plain comment. */
  replyTo: Comment | null;
  /** Puts the bar into reply mode for `comment` (each tap is a new choice, so the bar refills). */
  startReply: (comment: Comment) => void;
  cancelReply: () => void;
  /**
   * Likes someone else's post, or undoes the like (2026-11-24): the heart changes at once, and the
   * server ends with the last tap. Nothing on your own post or one you may not like. Returns the
   * failure line or null; none when the post went away, which is read again.
   */
  toggleLike: () => Promise<string | null>;
};

/**
 * One post (GET /posts/{entry_id}) for the post page and for the extras of your own entry page.
 * Reads again on focus, and on a return from the background once 20 minutes old (its photo links
 * stop working within an hour); what is on screen stays while a later read runs or fails. A 404
 * (deleted, a block, never visible) or an id that is not a UUID is `gone`. A ref keeps each action
 * to one request at a time; a like tapped while one is on its way waits for it, and then the
 * newest choice is sent.
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
  // One like request at a time; wantRef holds the heart the last tap asked for.
  const likingRef = useRef(false);
  const wantRef = useRef(false);
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
      // A read on its way may have begun before the comment left (a photo reload, a return from
      // the background); its answer would bring the comment back, so it is dropped.
      ++seqRef.current;
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

  // The heart as the server last answered it. A read that began while the like was on its way
  // would put the old heart back, so it is dropped.
  const settle = useCallback(
    (saved: boolean) => {
      ++seqRef.current;
      const now = postRef.current;
      if (mountedRef.current && now !== null && now.card.liked !== saved) put({ ...now, card: { ...now.card, liked: saved } });
    },
    [put]
  );

  const toggleLike = useCallback(async (): Promise<string | null> => {
    const now = postRef.current;
    if (now === null || now.card.mine || !now.can_react) return null;
    // A read already on its way began before the tap; its answer would put the old heart back.
    ++seqRef.current;
    wantRef.current = !now.card.liked;
    put({ ...now, card: { ...now.card, liked: wantRef.current } });
    // A request is on its way: it sends the newest choice when it answers.
    if (likingRef.current) return null;
    likingRef.current = true;
    let saved = now.card.liked;
    try {
      // It keeps sending after the page closes, so the server ends at the last tap.
      while (wantRef.current !== saved) saved = await setLike(id, wantRef.current);
      settle(saved);
      return null;
    } catch (err) {
      wantRef.current = saved;
      settle(saved);
      if (err instanceof ApiError && err.code === 'not_found') {
        // The post went away for you (deleted, a block, no longer followed): it is read again,
        // which turns the page into its gone line, so no notice.
        void load();
        return null;
      }
      return socialErrorMessage(err, 'like');
    } finally {
      likingRef.current = false;
    }
  }, [id, put, load, settle]);

  return { status, failure, post, reload: () => void load(), stale, comment, remove, drop, replyTo, startReply, cancelReply, toggleLike };
}
