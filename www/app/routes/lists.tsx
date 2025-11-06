// top-level imports
import { useLoaderData, useFetcher } from "react-router";
import type { Route } from "./+types/lists";
import { awesomeClient } from "~/api/client";
import type {
  Collection,
  Project,
  ProjectsStats,
} from "../proto/myawesomelist/v1/myawesomelist_pb";
import { useState, useEffect } from "react";
import { useIntersectionObserver, useTimeout } from "usehooks-ts";
import {
  ProjectStatsResponseSchema,
  ProjectStatsSchema,
} from "~/routes/projects.stats";
import { z } from "zod";

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

  // Zod schemas to validate and normalize collections shape
  const ProjectsStatsSchema = z.object({
    stargazersCount: z.coerce.number().int().nonnegative().optional(),
    openIssueCount: z.coerce.number().int().nonnegative().optional(),
  });

  const ProjectSchema = z.object({
    name: z.string().catch(""),
    url: z.string().catch(""),
    description: z.string().catch(""),
    stats: ProjectsStatsSchema.nullable().optional(),
  });

  const CategorySchema = z.object({
    name: z.string().catch(""),
    projects: z.array(ProjectSchema).catch([]),
  });

  const CollectionSchema = z.object({
    language: z.string().catch(""),
    categories: z.array(CategorySchema).catch([]),
  });

  const CollectionsSchema = z.array(CollectionSchema);

  // Validate and return a stable, serializable payload
  return CollectionsSchema.parse(res.collections ?? []);
}

export default function Lists() {
  const collections = useLoaderData<Collection[]>();

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
          {collections.map((collection, collIdx) => (
            <div key={`${collection.language}-${collIdx}`}>
              <div className="flex items-center mb-4">
                <span className="text-3xl mr-4">📚</span>
                <h2 className="text-2xl font-semibold text-gray-900 dark:text-white">
                  {collection.language}
                </h2>
              </div>

              {(collection.categories ?? []).map((category, catIdx) => (
                <section
                  key={`${collection.language}-${category.name}-${catIdx}`}
                >
                  <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-3">
                    {category.name}
                  </h3>

                  <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
                    {(category.projects ?? []).map((p, pIdx) => (
                      <ProjectCard
                        key={`${collection.language}-${category.name}-${p.name}-${p.url}-${pIdx}`}
                        project={p}
                      />
                    ))}
                  </div>
                </section>
              ))}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

// Fetch stats when visible
function ProjectCard({ project }: { project: Project }) {
  const [stats, setStats] = useState<z.infer<typeof ProjectStatsSchema> | null>(
    null,
  );
  const [errorToast, setErrorToast] = useState<string | null>(null);
  const fetcher = useFetcher<z.infer<typeof ProjectStatsResponseSchema>>();

  useTimeout(() => setErrorToast(null), errorToast ? 4000 : null);

  const { ref, isIntersecting } = useIntersectionObserver({
    threshold: 0.2,
    freezeOnceVisible: true,
  });

  useEffect(() => {
    if (!isIntersecting || stats) return;

    let owner: string | undefined;
    let repo: string | undefined;
    try {
      const parsed = parseGitHubOwnerRepoFromUrl(project.url ?? "");
      owner = parsed.owner;
      repo = parsed.repo;
    } catch {
      // Silently skip if URL is invalid
    }
    if (!owner || !repo) return;

    const params = new URLSearchParams({ owner, repo });
    fetcher.load(`/projects/stats?${params.toString()}`);
  }, [isIntersecting]);

  useEffect(() => {
    const parsed = ProjectStatsResponseSchema.safeParse(fetcher.data);
    if (!parsed.success) return;

    if ("error" in parsed.data) {
      setErrorToast(parsed.data.message);
      return;
    }

    if (parsed.data.stats) {
      setStats(parsed.data.stats);
    }
  }, [fetcher.data]);

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
        ref={ref}
        className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6"
        aria-busy={fetcher.state === "loading"}
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
          <div className="flex space-x-2 items-center">
            {fetcher.state === "loading" && (
              <span
                className="loading loading-spinner loading-sm text-blue-600"
                aria-label="Loading stats"
              />
            )}
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
          {fetcher.state === "loading" ? (
            <span className="skeleton h-4 w-28"></span>
          ) : stats?.stargazersCount ? (
            <span>⭐ {stats.stargazersCount.toLocaleString()} stars</span>
          ) : (
            <span>⭐ —</span>
          )}
          {fetcher.state === "loading" ? (
            <span className="skeleton h-4 w-24"></span>
          ) : stats?.openIssueCount ? (
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
  const u = new URL(url);
  if (u.hostname !== "github.com") return {};
  const parts = u.pathname.split("/").filter(Boolean);
  if (parts.length < 2) {
    throw new Error("Invalid GitHub URL");
  }
  return { owner: parts[0], repo: parts[1] };
}
