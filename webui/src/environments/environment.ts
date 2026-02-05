// Type definition for the injected config (also used in dev)
interface BitmagnetConfig {
  apiKey: string;
}

declare global {
  interface Window {
    __BITMAGNET_CONFIG__?: BitmagnetConfig;
  }
}

export const graphqlEndpoint = "http://localhost:3333/graphql";

// For development, you can set an API key here or via window.__BITMAGNET_CONFIG__
export function getApiKey(): string | undefined {
  return window.__BITMAGNET_CONFIG__?.apiKey;
}
