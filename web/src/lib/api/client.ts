import createClient from "openapi-fetch";

import type { components, paths } from "./schema";

// Typed client generated from api/openapi/efoy.yaml (`npm run gen:api`).
// Requests go to /api/efoy/*, which next.config.ts proxies to core-api.
export const api = createClient<paths>({ baseUrl: "/api/efoy" });

export type Schemas = components["schemas"];
