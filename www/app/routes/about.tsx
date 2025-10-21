import type { Route } from "./+types/about";

export function meta({}: Route.MetaArgs) {
  return [
    { title: "About - My Awesome List" },
    {
      name: "description",
      content:
        "Learn more about My Awesome List and our mission to curate the best resources.",
    },
  ];
}

export default function About() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <div className="container mx-auto px-4 py-8">
        <div className="max-w-4xl mx-auto">
          <div className="text-center mb-12">
            <h1 className="text-4xl font-bold text-gray-900 dark:text-white mb-4">
              About My Awesome List
            </h1>
            <p className="text-xl text-gray-600 dark:text-gray-300">
              Curating the best resources for developers, by developers
            </p>
          </div>

          <div className="grid md:grid-cols-2 gap-12 mb-12">
            <div>
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                Our Mission
              </h2>
              <p className="text-gray-600 dark:text-gray-300 mb-4">
                We believe that great software is built with great tools. Our
                mission is to curate and maintain comprehensive lists of the
                best resources, tools, and libraries across various domains of
                software development.
              </p>
              <p className="text-gray-600 dark:text-gray-300">
                Whether you're a beginner looking for learning resources or an
                experienced developer seeking the latest tools, we've got you
                covered.
              </p>
            </div>
            <div>
              <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
                How We Curate
              </h2>
              <ul className="space-y-3 text-gray-600 dark:text-gray-300">
                <li className="flex items-start">
                  <span className="text-green-500 mr-2">✓</span>
                  Community-driven submissions and reviews
                </li>
                <li className="flex items-start">
                  <span className="text-green-500 mr-2">✓</span>
                  Regular updates and maintenance
                </li>
                <li className="flex items-start">
                  <span className="text-green-500 mr-2">✓</span>
                  Quality over quantity approach
                </li>
                <li className="flex items-start">
                  <span className="text-green-500 mr-2">✓</span>
                  Active project and documentation requirements
                </li>
              </ul>
            </div>
          </div>

          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-8 mb-12">
            <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-6 text-center">
              By the Numbers
            </h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
              <div>
                <div className="text-3xl font-bold text-blue-600 dark:text-blue-400 mb-2">
                  50+
                </div>
                <div className="text-gray-600 dark:text-gray-300 text-sm">
                  Curated Lists
                </div>
              </div>
              <div>
                <div className="text-3xl font-bold text-green-600 dark:text-green-400 mb-2">
                  1000+
                </div>
                <div className="text-gray-600 dark:text-gray-300 text-sm">
                  Resources
                </div>
              </div>
              <div>
                <div className="text-3xl font-bold text-purple-600 dark:text-purple-400 mb-2">
                  10k+
                </div>
                <div className="text-gray-600 dark:text-gray-300 text-sm">
                  GitHub Stars
                </div>
              </div>
              <div>
                <div className="text-3xl font-bold text-orange-600 dark:text-orange-400 mb-2">
                  Daily
                </div>
                <div className="text-gray-600 dark:text-gray-300 text-sm">
                  Updates
                </div>
              </div>
            </div>
          </div>

          <div className="text-center">
            <h2 className="text-2xl font-semibold text-gray-900 dark:text-white mb-4">
              Get Involved
            </h2>
            <p className="text-gray-600 dark:text-gray-300 mb-6">
              Want to contribute? We welcome submissions, suggestions, and
              improvements from the community.
            </p>
            <div className="flex justify-center space-x-4">
              <a
                href="https://github.com/shikanime/myawesomelist"
                target="_blank"
                rel="noopener noreferrer"
                className="bg-gray-900 hover:bg-gray-800 text-white px-6 py-3 rounded-lg transition-colors duration-200"
              >
                View on GitHub
              </a>
              <a
                href="https://github.com/shikanime/myawesomelist/issues/new"
                target="_blank"
                rel="noopener noreferrer"
                className="bg-blue-600 hover:bg-blue-700 text-white px-6 py-3 rounded-lg transition-colors duration-200"
              >
                Submit a Resource
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
