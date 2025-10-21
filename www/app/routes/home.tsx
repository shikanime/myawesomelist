import { Link } from "react-router";
import type { Route } from "./+types/home";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "My Awesome List - Curated Lists of Awesome Things" },
    {
      name: "description",
      content:
        "Discover and explore curated lists of awesome resources, tools, and projects.",
    },
  ];
}

export default function Home() {
  return (
    <div className="min-h-screen bg-gradient-to-br from-blue-50 to-indigo-100 dark:from-gray-900 dark:to-gray-800">
      <div className="container mx-auto px-4 py-16">
        {/* Hero Section */}
        <div className="text-center mb-16">
          <h1 className="text-5xl font-bold text-gray-900 dark:text-white mb-6">
            My Awesome List
          </h1>
          <p className="text-xl text-gray-600 dark:text-gray-300 max-w-2xl mx-auto">
            Discover curated lists of awesome resources, tools, and projects
            across various domains. Find the best tools for your next project.
          </p>
        </div>

        {/* Featured Lists */}
        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-8 mb-16">
          {featuredLists.map((list) => (
            <div
              key={list.id}
              className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-6 hover:shadow-xl transition-shadow duration-300"
            >
              <div className="flex items-center mb-4">
                <span className="text-2xl mr-3">{list.icon}</span>
                <h3 className="text-xl font-semibold text-gray-900 dark:text-white">
                  {list.title}
                </h3>
              </div>
              <p className="text-gray-600 dark:text-gray-300 mb-4">
                {list.description}
              </p>
              <div className="flex justify-between items-center">
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  {list.itemCount} items
                </span>
                <button className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-md transition-colors duration-200">
                  Explore
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* Stats Section */}
        <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-8 text-center mb-16">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            <div>
              <div className="text-3xl font-bold text-blue-600 dark:text-blue-400 mb-2">
                50+
              </div>
              <div className="text-gray-600 dark:text-gray-300">
                Curated Lists
              </div>
            </div>
            <div>
              <div className="text-3xl font-bold text-green-600 dark:text-green-400 mb-2">
                1000+
              </div>
              <div className="text-gray-600 dark:text-gray-300">
                Awesome Resources
              </div>
            </div>
            <div>
              <div className="text-3xl font-bold text-purple-600 dark:text-purple-400 mb-2">
                Daily
              </div>
              <div className="text-gray-600 dark:text-gray-300">Updates</div>
            </div>
          </div>
        </div>

        {/* Call to Action */}
        <div className="text-center">
          <h2 className="text-3xl font-bold text-gray-900 dark:text-white mb-4">
            Ready to Explore?
          </h2>
          <p className="text-lg text-gray-600 dark:text-gray-300 mb-8 max-w-2xl mx-auto">
            Dive into our comprehensive collection of curated lists and discover
            the tools that will accelerate your development journey.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link
              to="/lists"
              className="bg-blue-600 hover:bg-blue-700 text-white px-8 py-3 rounded-lg text-lg font-semibold transition-colors duration-200"
            >
              Browse All Lists
            </Link>
            <Link
              to="/search"
              className="bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-900 dark:text-white px-8 py-3 rounded-lg text-lg font-semibold transition-colors duration-200"
            >
              Search Resources
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
}

const featuredLists = [
  {
    id: 1,
    title: "Frontend Development",
    description:
      "Essential tools and libraries for modern frontend development",
    icon: "🎨",
    itemCount: 45,
    slug: "frontend",
  },
  {
    id: 2,
    title: "Backend Development",
    description: "Server-side frameworks, databases, and deployment tools",
    icon: "⚙️",
    itemCount: 38,
    slug: "backend",
  },
  {
    id: 3,
    title: "DevOps & Infrastructure",
    description: "CI/CD, monitoring, and infrastructure management tools",
    icon: "🚀",
    itemCount: 32,
    slug: "devops",
  },
  {
    id: 4,
    title: "Machine Learning",
    description: "ML frameworks, datasets, and research resources",
    icon: "🤖",
    itemCount: 28,
    slug: "machine-learning",
  },
  {
    id: 5,
    title: "Mobile Development",
    description: "Native and cross-platform mobile development tools",
    icon: "📱",
    itemCount: 25,
    slug: "mobile",
  },
  {
    id: 6,
    title: "Security",
    description: "Cybersecurity tools, resources, and best practices",
    icon: "🔒",
    itemCount: 22,
    slug: "security",
  },
];
