import { Link } from "react-router";
import type { Route } from "./+types/lists.$category";

export function meta({ params }: Route.MetaArgs) {
  const categoryName = getCategoryName(params.category);
  return [
    { title: `${categoryName} Lists - My Awesome List` },
    {
      name: "description",
      content: `Curated awesome lists for ${categoryName.toLowerCase()}.`,
    },
  ];
}

export default function CategoryLists({ params }: Route.ComponentProps) {
  const category = params.category;
  const categoryData = getCategoryData(category);

  if (!categoryData) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-4">
            Category Not Found
          </h1>
          <Link
            to="/lists"
            className="text-blue-600 hover:text-blue-800 dark:text-blue-400"
          >
            ← Back to all lists
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-4 py-8">
        <div className="mb-8">
          <Link
            to="/lists"
            className="text-blue-600 hover:text-blue-800 dark:text-blue-400 mb-4 inline-block"
          >
            ← Back to all lists
          </Link>
          <div className="flex items-center mb-4">
            <span className="text-4xl mr-4">{categoryData.icon}</span>
            <h1 className="text-4xl font-bold text-gray-900 dark:text-white">
              {categoryData.name}
            </h1>
          </div>
          <p className="text-lg text-gray-600 dark:text-gray-300">
            {categoryData.description}
          </p>
        </div>

        <div className="grid gap-6">
          {categoryData.lists.map((list) => (
            <div
              key={list.id}
              className="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6"
            >
              <div className="flex justify-between items-start mb-4">
                <div>
                  <h3 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
                    {list.title}
                  </h3>
                  <p className="text-gray-600 dark:text-gray-300 mb-4">
                    {list.description}
                  </p>
                </div>
                <div className="flex space-x-2">
                  <a
                    href={list.githubUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="bg-gray-900 hover:bg-gray-800 text-white px-4 py-2 rounded-md text-sm transition-colors duration-200"
                  >
                    GitHub
                  </a>
                  {list.websiteUrl && (
                    <a
                      href={list.websiteUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md text-sm transition-colors duration-200"
                    >
                      Website
                    </a>
                  )}
                </div>
              </div>
              <div className="flex flex-wrap gap-2 mb-4">
                {list.tags.map((tag) => (
                  <span
                    key={tag}
                    className="bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-2 py-1 rounded-md text-xs"
                  >
                    {tag}
                  </span>
                ))}
              </div>
              <div className="flex justify-between items-center text-sm text-gray-500 dark:text-gray-400">
                <span>⭐ {list.stars.toLocaleString()} stars</span>
                <span>Updated {list.lastUpdated}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function getCategoryName(slug: string): string {
  const categories: Record<string, string> = {
    frontend: "Frontend Development",
    backend: "Backend Development",
    devops: "DevOps & Infrastructure",
    "machine-learning": "Machine Learning",
    mobile: "Mobile Development",
    security: "Security",
    "data-science": "Data Science",
    "game-dev": "Game Development",
    design: "Design & UX",
  };
  return categories[slug] || "Unknown Category";
}

function getCategoryData(slug: string) {
  const categories: Record<string, any> = {
    frontend: {
      name: "Frontend Development",
      icon: "🎨",
      description:
        "Essential tools and libraries for modern frontend development",
      lists: [
        {
          id: 1,
          title: "Awesome React",
          description:
            "A collection of awesome things regarding React ecosystem",
          githubUrl: "https://github.com/enaqx/awesome-react",
          websiteUrl: null,
          tags: ["react", "javascript", "frontend"],
          stars: 58000,
          lastUpdated: "2 days ago",
        },
        {
          id: 2,
          title: "Awesome Vue.js",
          description: "A curated list of awesome things related to Vue.js",
          githubUrl: "https://github.com/vuejs/awesome-vue",
          websiteUrl: "https://awesome-vue.js.org/",
          tags: ["vue", "javascript", "frontend"],
          stars: 71000,
          lastUpdated: "1 week ago",
        },
        {
          id: 3,
          title: "Awesome CSS",
          description: "A curated contents of amazing CSS",
          githubUrl: "https://github.com/awesome-css-group/awesome-css",
          websiteUrl: null,
          tags: ["css", "styling", "frontend"],
          stars: 17000,
          lastUpdated: "3 days ago",
        },
      ],
    },
    backend: {
      name: "Backend Development",
      icon: "⚙️",
      description: "Server-side frameworks, databases, and deployment tools",
      lists: [
        {
          id: 4,
          title: "Awesome Node.js",
          description: "Delightful Node.js packages and resources",
          githubUrl: "https://github.com/sindresorhus/awesome-nodejs",
          websiteUrl: null,
          tags: ["nodejs", "javascript", "backend"],
          stars: 55000,
          lastUpdated: "1 day ago",
        },
        {
          id: 5,
          title: "Awesome Python",
          description:
            "A curated list of awesome Python frameworks, libraries, software and resources",
          githubUrl: "https://github.com/vinta/awesome-python",
          websiteUrl: "https://awesome-python.com/",
          tags: ["python", "backend", "frameworks"],
          stars: 190000,
          lastUpdated: "5 days ago",
        },
      ],
    },
  };
  return categories[slug];
}
