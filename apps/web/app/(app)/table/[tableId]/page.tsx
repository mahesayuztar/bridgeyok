import { requireAccount } from "../../../account-server";

export default async function TablePage({ params }: { params: Promise<{ tableId: string }> }) {
  const { tableId } = await params;
  await requireAccount(`/table/${tableId}`);
  return null;
}
