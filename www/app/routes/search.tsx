import { useState } from "react";
import type { Route } from "./+types/search";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "Search - My Awesome List" },
    {
      name: "description",
      content: "Search through all awesome lists and resources.",
    },
  ];
}

export default function Search() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<any[]>([]);

  const handleSearch = (searchQuery: string) => {
    setQuery(searchQuery);
    // Mock search results - in a real app, this would be an API call
    if (searchQuery.trim()) {
      const mockResults = [
        {
          id: 1,
          title: "Awesome React",
          description:
            "A collection of awesome things regarding React ecosystem",
          category: "Frontend Development",
          tags: ["react", "javascript", "frontend"],
          stars: 58000,
        },
        {
          id: 2,
          title: "Awesome Python",
          description:
            "A curated list of awesome Python frameworks, libraries, software and resources",
          category: "Backend Development",
          tags: ["python", "backend", "frameworks"],
          stars: 190000,
        },
      ].filter(
        (item) =>
          item.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
          item.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
          item.tags.some((tag) =>
            tag.toLowerCase().includes(searchQuery.toLowerCase()),
          ),
      );
      setResults(mockResults);
    } else {
      setResults([]);
    }
  };

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
            <div className="relative">
              <input
                type="text"
                placeholder="Search for lists, tools, or technologies..."
                value={query}
                onChange={(e) => handleSearch(e.target.value)}
                className="w-full px-4 py-3 pl-12 text-lg border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-800 text-gray-900 dark:text-white"
              />
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
          </div>
        </div>

        {query && (
          <div className="mb-6">
            <p className="text-gray-600 dark:text-gray-300">
              {results.length} results for "{query}"
            </p>
          </div>
        )}

        <div className="grid gap-6">
          {results.map((result) => (
            <div
              key={result.id}
              className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6"
            >
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
                    {result.title}
                  </h3>
                  <p className="text-gray-600 dark:text-gray-300 mb-2">
                    {result.description}
                  </p>
                  <p className="text-sm text-blue-600 dark:text-blue-400">
                    {result.category}
                  </p>
                </div>
                <div className="text-right">
                  <div className="text-sm text-gray-500 dark:text-gray-400">
                    ⭐ {result.stars.toLocaleString()} stars
                  </div>
                </div>
              </div>
              <div className="flex flex-wrap gap-2">
                {result.tags.map((tag: string) => (
                  <span
                    key={tag}
                    className="bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-2 py-1 rounded-md text-xs"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>

        {query && results.length === 0 && (
          <div className="text-center py-12">
            <div className="text-gray-400 text-6xl mb-4">🔍</div>
            <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
              No results found
            </h3>
            <p className="text-gray-600 dark:text-gray-300">
              Try searching with different keywords or browse our categories
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
