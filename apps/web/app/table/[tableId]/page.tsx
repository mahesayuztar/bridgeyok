import { requireAccount } from "../../account-server";
import { AccountPresence } from "../../account-presence";
import { TableSocialProvider } from "../table-social";
import BridgeTable from "../../bridge-table";

export default async function TablePage({ params }: { params: Promise<{ tableId: string }> }) {
  const { tableId } = await params;

  const account = await requireAccount(`/table/${tableId}`);
  return <><AccountPresence compact /><TableSocialProvider tableId={tableId} viewerId={account.profile.id}><BridgeTable expectedTableId={tableId} /></TableSocialProvider></>;
}
