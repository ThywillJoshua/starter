## Pulse Help Center

Mintlify project that powers the Pulse Help tab embedded in ClaraDocs.

### Development

```bash
# install Mintlify CLI once
bun install -g mint

# run the docs locally
cd pulse-docs
mint dev
```

The site is intentionally single-tab: all content lives inside `/help`. Update `docs.json` to add navigation groups or tweak theming, and add/edit articles inside `help/<category>/...`.
