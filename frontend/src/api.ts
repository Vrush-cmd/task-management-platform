export type Task = {
  id: string;
  title: string;
  description: string;
  completed: boolean;
  created_at: string;
  updated_at: string;
};

type TaskInput = Pick<Task, "title" | "description">;

const API_BASE = import.meta.env.VITE_API_URL ?? "/api/v1";

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });

  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new Error(body?.error ?? `Request failed with status ${response.status}`);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}

export const taskApi = {
  list: () => request<Task[]>("/tasks"),
  create: (input: TaskInput) =>
    request<Task>("/tasks", { method: "POST", body: JSON.stringify(input) }),
  toggle: (id: string) => request<Task>(`/tasks/${id}/complete`, { method: "PATCH" }),
  remove: (id: string) => request<void>(`/tasks/${id}`, { method: "DELETE" }),
};
