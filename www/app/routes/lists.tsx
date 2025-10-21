import { Link } from "react-router";
import type { Route } from "./+types/lists";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "All Lists - My Awesome List" },
    {
      name: "description",
      content: "Browse all curated awesome lists by category.",
    },
  ];
}

export default function Lists() {
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

        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
          {categories.map((category) => (
            <Link
              key={category.slug}
              to={`/lists/${category.slug}`}
              className="block bg-white dark:bg-gray-800 rounded-lg shadow-md hover:shadow-lg transition-shadow duration-300 p-6"
            >
              <div className="flex items-center mb-4">
                <span className="text-3xl mr-4">{category.icon}</span>
                <div>
                  <h3 className="text-xl font-semibold text-gray-900 dark:text-white">
                    {category.name}
                  </h3>
                  <p className="text-sm text-gray-500 dark:text-gray-400">
                    {category.count} lists
                  </p>
                </div>
              </div>
              <p className="text-gray-600 dark:text-gray-300 text-sm">
                {category.description}
              </p>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

const categories = [
  {
    name: "Frontend Development",
    slug: "frontend",
    icon: "🎨",
    count: 12,
    description: "React, Vue, Angular, CSS frameworks, and frontend tools",
  },
  {
    name: "Backend Development",
    slug: "backend",
    icon: "⚙️",
    count: 15,
    description: "Node.js, Python, Go, databases, and server technologies",
  },
  {
    name: "DevOps & Infrastructure",
    slug: "devops",
    icon: "🚀",
    count: 8,
    description: "Docker, Kubernetes, CI/CD, monitoring, and cloud services",
  },
  {
    name: "Machine Learning",
    slug: "machine-learning",
    icon: "🤖",
    count: 10,
    description: "ML frameworks, datasets, research papers, and AI tools",
  },
  {
    name: "Mobile Development",
    slug: "mobile",
    icon: "📱",
    count: 7,
    description: "React Native, Flutter, iOS, Android development tools",
  },
  {
    name: "Security",
    slug: "security",
    icon: "🔒",
    count: 6,
    description: "Cybersecurity tools, penetration testing, and best practices",
  },
  {
    name: "Data Science",
    slug: "data-science",
    icon: "📊",
    count: 9,
    description: "Data analysis, visualization, statistics, and research tools",
  },
  {
    name: "Game Development",
    slug: "game-dev",
    icon: "🎮",
    count: 5,
    description: "Game engines, graphics libraries, and game development tools",
  },
  {
    name: "Design & UX",
    slug: "design",
    icon: "🎯",
    count: 8,
    description: "Design tools, UI/UX resources, and creative software",
  },
];
