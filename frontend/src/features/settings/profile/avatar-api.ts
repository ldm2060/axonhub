import { getTokenFromStorage } from '@/stores/authStore';

export async function uploadAvatar(file: File): Promise<{ avatar: string }> {
  const data = new FormData();
  data.append('file', file);

  const token = getTokenFromStorage();
  const response = await fetch('/admin/me/avatar', {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: data,
  });

  if (!response.ok) {
    const payload = (await response.json().catch(() => null)) as { error?: { message?: string } } | null;
    throw new Error(payload?.error?.message || `Avatar upload failed (${response.status})`);
  }

  return (await response.json()) as { avatar: string };
}
