import { redirect } from "next/navigation";

export default function MaterialDetailPage({ params }: { params: { id: string } }) {
  const id = params?.id;
  redirect(`/material/knowledge/${id}`);
}
