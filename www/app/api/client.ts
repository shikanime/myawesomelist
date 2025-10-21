import { createConnectTransport } from "@connectrpc/connect-web";
import { createClient } from "@connectrpc/connect";
import { AwesomeService } from "~/proto/myawesomelist/v1/myawesomelist_pb";

const baseUrl = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

const transport = createConnectTransport({
  baseUrl,
});

export const awesomeClient = createClient(AwesomeService, transport);
