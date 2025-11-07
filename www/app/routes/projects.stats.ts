import { awesomeClient } from "~/api/client";
import type { Route } from "./+types/projects.stats";
import { z } from "zod";
import type { ProjectStats } from "~/proto/myawesomelist/v1/myawesomelist_pb";

export const SearchParamsSchema = z.object({
  hostname: z.string().min(1, "hostname is required"),
  owner: z.string().min(1, "owner is required"),
  repo: z.string().min(1, "repo is required"),
});

export const ProjectStatsSchema: z.ZodType<ProjectStats> = z.any();

export const StatsSuccessSchema = z.object({
  stats: ProjectStatsSchema,
});

export const ErrorPayloadSchema = z.object({
  error: z.string(),
  message: z.string(),
  issues: z.any().optional(),
});

export const ProjectStatsResponseSchema = z.union([
  StatsSuccessSchema,
  ErrorPayloadSchema,
]);

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const sp = SearchParamsSchema.safeParse(
    Object.fromEntries(url.searchParams.entries()),
  );

  if (!sp.success) {
    return new Response(
      JSON.stringify(
        ErrorPayloadSchema.parse({
          error: "invalid query",
          message: sp.error.message,
          issues: sp.error.issues,
        }),
      ),
      {
        status: 400,
        headers: { "Content-Type": "application/json" },
      },
    );
  }

  try {
    return StatsSuccessSchema.parse(
      await awesomeClient.getProjectStats({ repo: sp.data }),
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
