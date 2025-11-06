import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("/lists", "routes/lists.tsx"),
  route("/search", "routes/search.tsx"),
  route("/about", "routes/about.tsx"),
  route("/projects/stats", "routes/projects.stats.ts"),
] satisfies RouteConfig;
