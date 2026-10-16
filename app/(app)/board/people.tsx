import { PeopleScreen } from '@/components/people/PeopleScreen';

/** People, opened from the Board's 'Find people' and its person-add button: Back reads 'Board'. */
export default function BoardPeopleRoute() {
  return <PeopleScreen from="board" />;
}
