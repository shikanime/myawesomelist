import type { Route } from "./+types/search";
import { useFetcher } from "react-router";
import { awesomeClient } from "~/api/client";
import { z } from "zod";
import { ProjectCard } from "~/components/ProjectCard";
import type { Project } from "~/proto/myawesomelist/v1/myawesomelist_pb";

export const SearchParamsSchema = z.object({
  query: z.string().optional().default(""),
  limit: z.coerce.number().int().positive().max(100).default(20),
});

export const ProjectZodSchema: z.ZodType<Project> = z.any();

export const SearchSuccessPayloadSchema = z.object({
  projects: z.array(ProjectZodSchema),
});

export const ErrorPayloadSchema = z.object({
  error: z.string(),
  message: z.string(),
  issues: z.any().optional(),
});

export const SearchResponseSchema = z.union([
  SearchSuccessPayloadSchema,
  ErrorPayloadSchema,
]);

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Search - My Awesome List" },
    {
      name: "description",
      content: "Search through all awesome lists and resources.",
    },
  ];
}

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const parsed = SearchParamsSchema.safeParse(
    Object.fromEntries(url.searchParams.entries()),
  );

  if (!parsed.success) {
    const payload = ErrorPayloadSchema.parse({
      error: "invalid query",
      message: parsed.error.message,
      issues: parsed.error.issues,
    });
    return new Response(JSON.stringify(payload), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  if (!parsed.data.query) {
    return [];
  }

  try {
    return SearchSuccessPayloadSchema.parse(
      await awesomeClient.searchProjects({
        query: parsed.data.query,
        limit: parsed.data.limit,
        repos: [],
      }),
    );
  } catch (err) {
    return new Response(
      JSON.stringify(
        ErrorPayloadSchema.parse({
          error: "backend error",
          message: String(err),
        }),
      ),
      {
        status: 502,
        headers: { "Content-Type": "application/json" },
      },
    );
  }
}

export default function Search() {
  const fetcher = useFetcher<z.infer<typeof SearchResponseSchema>>();

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
            Search Awesome Lists
          </h1>
          <p className="text-lg text-gray-600 dark:text-gray-300 mb-6">
            Find the perfect resources for your project
          </p>

          <div className="max-w-2xl">
            <fetcher.Form method="get">
              <div className="relative">
                <input
                  type="text"
                  name="query"
                  placeholder="Search for lists, tools, or technologies..."
                  className="w-full px-4 py-3 pl-12 text-lg border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
                />
                <input type="hidden" name="limit" value="20" />
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <svg
                    className="h-6 w-6 text-gray-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                    />
                  </svg>
                </div>
              </div>
            </fetcher.Form>

            {fetcher.data && "error" in fetcher.data && (
              <div
                className="alert alert-error mt-4"
                role="alert"
                aria-live="polite"
              >
                <span>{fetcher.data.message}</span>
              </div>
            )}
          </div>
        </div>

        <div className="grid gap-6" aria-busy={fetcher.state === "loading"}>
          {fetcher.state === "loading" && (
            <div className="flex justify-center py-6">
              <span
                className="loading loading-spinner loading-md text-blue-600"
                aria-label="Searching"
              />
            </div>
          )}

          {fetcher.data &&
            "projects" in fetcher.data &&
            fetcher.data.projects.length === 0 && (
              <p className="text-gray-600 dark:text-gray-300">
                No results found.
              </p>
            )}

          {fetcher.data &&
            "projects" in fetcher.data &&
            fetcher.data.projects.map((p) => (
              <ProjectCard
                key={`${p.repo?.hostname}-${p.repo?.owner}-${p.repo?.repo}-${p.name}`}
                project={p}
              />
            ))}
        </div>
      </div>
    </div>
  );
}
