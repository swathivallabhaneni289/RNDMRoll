import { useRouter } from 'expo-router';
import { BoardScreen } from '@/components/board/BoardScreen';
import { useSession } from '@/lib/session/store';
import { readToday } from '@/lib/wheel/todayStore';
import { refetchToday } from '@/lib/wheel/useToday';

/**
 * The Board tab's first page. Your own card opens your entry page in this stack (Back reads
 * 'Board'); anyone else's opens their post. The locked card's 'Log your ...' makes sure Today is
 * fresh and opens the entry screen only when there is a spin to post; 'Spin first' and
 * 'Go to Spin' go to the Spin page.
 */
export default function BoardRoute() {
  const router = useRouter();
  const { user } = useSession();
  const userId = user?.id ?? null;

  async function onLog() {
    if (userId === null) return;
    await refetchToday(userId, 'manual');
    const state = readToday(userId).today?.state ?? null;
    if (state === 'result' || state === 'late') router.push('/(app)/today/log');
    else router.navigate('/(app)/today');
  }

  return (
    <BoardScreen
      onFindPeople={() => router.push('/(app)/board/people')}
      onOpenCard={(card) =>
        card.mine
          ? router.push({ pathname: '/(app)/board/entry', params: { id: card.entry_id } })
          : router.push({ pathname: '/(app)/board/post', params: { id: card.entry_id } })
      }
      onLog={() => void onLog()}
      onSpin={() => router.navigate('/(app)/today')}
    />
  );
}
