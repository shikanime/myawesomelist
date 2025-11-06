import { useLoaderData } from "react-router";
import type { Route } from "./+types/lists";
import { awesomeClient } from "../api/client";
import type {
  Category,
  Project,
  ProjectsStats,
} from "../proto/myawesomelist/v1/myawesomelist_pb";
import { useRef, useState, useEffect } from "react";
import { useIntersectionObserver } from "../hooks/intersectionObserver";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Lists - My Awesome List" },
    {
      name: "description",
      content: "Browse all curated awesome lists by category.",
    },
  ];
}

export async function loader(_: Route.LoaderArgs) {
  const res = await awesomeClient.listCollections({});
  const collections = res.collections ?? [];
  const categories = collections.flatMap((c) => c.categories ?? []);
  return categories;
}

export default function Lists() {
  const categories = useLoaderData<Category[]>();

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
            All Awesome Lists
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-300">
            Browse our curated collection of awesome lists organized by
            category.
          </p>
        </div>

        <div className="grid gap-12">
          {categories.map((category) => (
            <section key={category.name}>
              <div className="flex items-center mb-4">
                <span className="text-3xl mr-4">📚</span>
                <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">
                  {category.name}
                </h2>
              </div>

              <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
                {(category.projects ?? []).map((p) => (
                  <ProjectCard key={`${p.name}-${p.url}`} project={p} />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}

// Fetch stats when visible
function ProjectCard({ project }: { project: Project }) {
  const [stats, setStats] = useState<ProjectsStats | undefined>(project.stats);
  const cardRef = useRef<HTMLDivElement | null>(null);
  const [errorToast, setErrorToast] = useState<string | null>(null);

  useEffect(() => {
    if (!errorToast) return;
    const id = setTimeout(() => setErrorToast(null), 4000);
    return () => clearTimeout(id);
  }, [errorToast]);

  useIntersectionObserver(
    cardRef,
    () => {
      const { owner, repo } = parseGitHubOwnerRepoFromUrl(project.url ?? "");
      if (!owner || !repo) return;
      awesomeClient
        .getProjectStats({ repo: { owner, repo } })
        .then((res) => setStats(res.stats))
        .catch(() => {
          setErrorToast(`Failed to load stats for ${owner}/${repo}`);
        });
    },
    { threshold: 0.2 },
  );

  return (
    <>
      {errorToast && (
        <div className="toast toast-top toast-end">
          <div className="alert alert-error" role="alert">
            <span>{errorToast}</span>
          </div>
        </div>
      )}
      <div
        ref={cardRef}
        className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6"
      >
        <div className="flex justify-between items-start mb-4">
          <div>
            <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
              {project.name}
            </h3>
            <p className="text-gray-600 dark:text-gray-300 mb-4">
              {project.description}
            </p>
          </div>
          <div className="flex space-x-2">
            <a
              href={project.url}
              target="_blank"
              rel="noopener noreferrer"
              className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md text-sm transition-colors duration-200"
            >
              Visit
            </a>
          </div>
        </div>
        <div className="flex justify-between items-center text-sm text-gray-500 dark:text-gray-400">
          {stats?.stargazersCount ? (
            <span>⭐ {stats.stargazersCount.toLocaleString()} stars</span>
          ) : (
            <span>⭐ —</span>
          )}
          {stats?.openIssueCount ? (
            <span>{stats.openIssueCount} open issues</span>
          ) : (
            <span>Issues —</span>
          )}
        </div>
      </div>
    </>
  );
}

function parseGitHubOwnerRepoFromUrl(url: string): {
  owner?: string;
  repo?: string;
} {
  try {
    const u = new URL(url);
    if (u.hostname !== "github.com") return {};
    const parts = u.pathname.split("/").filter(Boolean);
    if (parts.length >= 2) {
      return { owner: parts[0], repo: parts[1] };
    }
  } catch {
    // ignore invalid URLs
  }
  return {};
}
