import { awesomeClient } from "~/api/client";
import type { Route } from "./+types/projects.stats";
import { z } from "zod";

export const SearchParamsSchema = z.object({
  owner: z.string().min(1, "owner is required"),
  repo: z.string().min(1, "repo is required"),
});

export const ProjectStatsSchema = z.object({
  stargazersCount: z.coerce.number().int().nonnegative().default(0),
  openIssueCount: z.coerce.number().int().nonnegative().default(0),
});

export const StatsSuccessSchema = z.object({
  stats: ProjectStatsSchema.nullable(),
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
    const payload = ErrorPayloadSchema.parse({
      error: "invalid query",
      message: sp.error.message,
      issues: sp.error.issues,
    });
    return new Response(JSON.stringify(payload), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  try {
    const payload = StatsSuccessSchema.parse(
      await awesomeClient.getProjectStats({ repo: sp.data }),
    );

    return new Response(JSON.stringify(payload), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  } catch (err) {
    const payload = ErrorPayloadSchema.parse({
      error: "backend error",
      message: String(err),
    });
    return new Response(JSON.stringify(payload), {
      status: 502,
      headers: { "Content-Type": "application/json" },
    });
  }
}
