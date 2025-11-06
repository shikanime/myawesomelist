import { useLoaderData } from "react-router";
import type { Route } from "./+types/lists";
import { awesomeClient } from "~/api/client";
import type { Collection } from "~/proto/myawesomelist/v1/myawesomelist_pb";
import { ProjectCard } from "~/components/ProjectCard";
import { z } from "zod";

export const CollectionSchema: z.ZodType<Collection> = z.any();

export const ListsSuccessSchema = z.object({
  collections: z.array(CollectionSchema),
});

export const ListsResponseSchema = z.union([ListsSuccessSchema]);

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
  return ListsResponseSchema.parse(res);
}

export default function Lists() {
  const { collections } = useLoaderData<z.infer<typeof ListsSuccessSchema>>();

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
                        key={`${collection.language}-${category.name}-${p.name}-${p.repo?.hostname}-${p.repo?.owner}-${p.repo?.repo}-${pIdx}`}
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
