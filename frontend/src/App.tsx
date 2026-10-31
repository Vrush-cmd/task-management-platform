import { FormEvent, useEffect, useMemo, useState, type CSSProperties } from "react";
import { Check, ClipboardList, Plus, RefreshCw, Sparkles, Trash2 } from "lucide-react";
import { Task, taskApi } from "./api";

type Filter = "all" | "active" | "completed";

function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [error, setError] = useState("");

  const loadTasks = async () => {
    setLoading(true);
    setError("");
    try {
      setTasks(await taskApi.list());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to load tasks.");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadTasks();
  }, []);

  const filteredTasks = useMemo(() => {
    if (filter === "active") return tasks.filter((task) => !task.completed);
    if (filter === "completed") return tasks.filter((task) => task.completed);
    return tasks;
  }, [filter, tasks]);

  const completedCount = tasks.filter((task) => task.completed).length;
  const activeCount = tasks.length - completedCount;

  const addTask = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!title.trim()) return;

    setSaving(true);
    setError("");
    try {
      const task = await taskApi.create({ title: title.trim(), description: description.trim() });
      setTasks((current) => [task, ...current]);
      setTitle("");
      setDescription("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to create task.");
    } finally {
      setSaving(false);
    }
  };

  const toggleTask = async (task: Task) => {
    setBusyId(task.id);
    setError("");
    try {
      const updated = await taskApi.toggle(task.id);
      setTasks((current) => current.map((item) => (item.id === task.id ? updated : item)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to update task.");
    } finally {
      setBusyId(null);
    }
  };

  const deleteTask = async (task: Task) => {
    setBusyId(task.id);
    setError("");
    try {
      await taskApi.remove(task.id);
      setTasks((current) => current.filter((item) => item.id !== task.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to delete task.");
    } finally {
      setBusyId(null);
    }
  };

  return (
    <main className="app-shell">
      <div className="ambient ambient-one" />
      <div className="ambient ambient-two" />

      <section className="workspace">
        <header className="topbar">
          <div className="brand">
            <div className="brand-mark"><Sparkles size={18} /></div>
            <span>TaskFlow</span>
          </div>
          <button className="icon-button" onClick={() => void loadTasks()} aria-label="Refresh tasks" title="Refresh tasks">
            <RefreshCw size={18} />
          </button>
        </header>

        <section className="hero">
          <div>
            <p className="eyebrow">YOUR PERSONAL WORKSPACE</p>
            <h1>Make space for<br /><em>what matters.</em></h1>
            <p className="hero-copy">A calm place to capture the work ahead<br />and keep your momentum.</p>
          </div>
          <div className="progress-card">
            <div className="progress-ring" style={{ "--progress": `${tasks.length ? (completedCount / tasks.length) * 100 : 0}%` } as CSSProperties}>
              <span>{completedCount}<small>/{tasks.length}</small></span>
            </div>
            <div><strong>{completedCount ? "Nice progress" : "Ready when you are"}</strong><span>{completedCount ? `${activeCount} left to go` : "Start with one small thing"}</span></div>
          </div>
        </section>

        <form className="composer" onSubmit={addTask}>
          <div className="composer-icon"><Plus size={20} /></div>
          <div className="composer-fields">
            <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="What needs to be done?" maxLength={200} aria-label="Task title" />
            <input className="description-input" value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a little context (optional)" aria-label="Task description" />
          </div>
          <button className="add-button" type="submit" disabled={saving || !title.trim()}>{saving ? "Adding…" : "Add task"}</button>
        </form>

        {error && <div className="error-banner" role="alert">{error}</div>}

        <div className="list-heading">
          <div><h2>My tasks</h2><span>{activeCount} active · {completedCount} completed</span></div>
          <div className="filters" role="tablist" aria-label="Task filters">
            {(["all", "active", "completed"] as Filter[]).map((option) => <button key={option} className={filter === option ? "selected" : ""} onClick={() => setFilter(option)} type="button">{option}</button>)}
          </div>
        </div>

        <section className="task-list" aria-live="polite">
          {loading ? <div className="empty-state"><RefreshCw className="spin" size={24} /><p>Gathering your tasks…</p></div> : filteredTasks.length === 0 ? <div className="empty-state"><ClipboardList size={28} /><p>{tasks.length ? "Nothing in this view." : "Your list is clear. Add your first task above."}</p></div> : filteredTasks.map((task) => <article className={`task-card ${task.completed ? "is-complete" : ""}`} key={task.id}>
            <button className="check-button" onClick={() => void toggleTask(task)} disabled={busyId === task.id} aria-label={task.completed ? `Mark ${task.title} incomplete` : `Complete ${task.title}`}>
              {task.completed && <Check size={15} strokeWidth={3} />}
            </button>
            <div className="task-content"><h3>{task.title}</h3>{task.description && <p>{task.description}</p>}<time>{task.completed ? "Completed" : `Added ${formatDate(task.created_at)}`}</time></div>
            <button className="delete-button" onClick={() => void deleteTask(task)} disabled={busyId === task.id} aria-label={`Delete ${task.title}`}><Trash2 size={17} /></button>
          </article>)}
        </section>

        <footer><span>TaskFlow</span><span>One thing at a time.</span></footer>
      </section>
    </main>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" }).format(new Date(value));
}

export default App;
