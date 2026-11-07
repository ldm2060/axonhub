// Synapse mark: four endpoints joined by concave arcs through a shared hub.
// Arcs and side nodes follow `currentColor`; top/bottom nodes use `--logo-accent`
// (champagne gold, tuned per light/dark in index.css) so the mark fits every theme.
export const AXONHUB_LOGO_ARCS =
  'M32 7V15C32 24 24 32 15 32H7M32 15C32 24 40 32 49 32H57M32 57V49C32 40 24 32 15 32M32 49C32 40 40 32 49 32';

export const AXONHUB_LOGO_NODES = {
  accent: [
    [32, 7],
    [32, 57],
  ],
  base: [
    [7, 32],
    [57, 32],
  ],
} as const;
