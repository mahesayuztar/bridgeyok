import BridgeTable from "../../bridge-table";

export default async function TablePage({ params }: { params: Promise<{ tableId: string }> }) {
  const { tableId } = await params;

  return <BridgeTable expectedTableId={tableId} />;
}
