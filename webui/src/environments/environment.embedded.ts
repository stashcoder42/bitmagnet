// Type definition for the injected config
interface BitmagnetConfig {
  apiKey: string;
}

declare global {
  interface Window {
    __BITMAGNET_CONFIG__?: BitmagnetConfig;
  }
}

export const graphqlEndpoint =
  window.location.protocol +
  "//" +
  window.location.hostname +
  ":" +
  window.location.port +
  "/graphql";

export function getApiKey(): string | undefined {
  return window.__BITMAGNET_CONFIG__?.apiKey;
}
