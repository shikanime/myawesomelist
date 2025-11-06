import { useLoaderData } from "react-router";
import type { Route } from "./+types/lists";
import { awesomeClient } from "../api/client";
import type { Category } from "../proto/myawesomelist/v1/myawesomelist_pb";

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
  const res = await awesomeClient.listCollections({  });
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
                  <div
                    key={`${p.name}-${p.url}`}
                    className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6"
                  >
                    <div className="flex justify-between items-start mb-4">
                      <div>
                        <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
                          {p.name}
                        </h3>
                        <p className="text-gray-600 dark:text-gray-300 mb-4">
                          {p.description}
                        </p>
                      </div>
                      <div className="flex space-x-2">
                        <a
                          href={p.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md text-sm transition-colors duration-200"
                        >
                          Visit
                        </a>
                      </div>
                    </div>
                    <div className="flex justify-between items-center text-sm text-gray-500 dark:text-gray-400">
                      {p.stats?.stargazersCount ? (
                        <span>
                          ⭐ {p.stats.stargazersCount.toLocaleString()} stars
                        </span>
                      ) : (
                        <span>⭐ —</span>
                      )}
                      {p.stats?.openIssueCount ? (
                        <span>{p.stats.openIssueCount} open issues</span>
                      ) : (
                        <span>Issues —</span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  );
}
