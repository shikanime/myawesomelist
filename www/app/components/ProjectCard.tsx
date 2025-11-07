import { useState, useEffect } from "react";
import { useIntersectionObserver, useTimeout } from "usehooks-ts";
import {
  ProjectStatsResponseSchema,
  ProjectStatsSchema,
} from "~/routes/projects.stats";
import { z } from "zod";
import { useFetcher } from "react-router";
import type { Project } from "~/proto/myawesomelist/v1/myawesomelist_pb";

export function ProjectCard({ project }: { project: Project }) {
  const [stats, setStats] = useState<z.infer<typeof ProjectStatsSchema> | null>(
    null,
  );
  const [errorToast, setErrorToast] = useState<string | null>(null);
  const fetcher = useFetcher<typeof ProjectStatsResponseSchema>();

  useTimeout(() => setErrorToast(null), errorToast ? 4000 : null);

  const { ref, isIntersecting } = useIntersectionObserver({
    threshold: 0.2,
    freezeOnceVisible: true,
  });

  useEffect(() => {
    if (!isIntersecting || stats) return;
    const params = new URLSearchParams({
      hostname: project.repo?.hostname ?? "",
      owner: project.repo?.owner ?? "",
      repo: project.repo?.repo ?? "",
    });
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
              href={`https://${project.repo?.hostname}/${project.repo?.owner}/${project.repo?.repo}`}
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
