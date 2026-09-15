---
title: Docs Anchors
---

# Docs Anchors

In-page markdown fragment links (`[text](#heading-id)` and
`[text](other.md#heading-id)`) under `docs/` are checked against **VitePress
heading ids**, not lychee's independent slug guess.

## Why this matters

lychee's `--include-fragments` mode slugifies headings with a different
algorithm than VitePress's `markdown-it-anchor` plugin (which uses
`@mdit-vue/shared` slugify). A heading such as
`config.base.yaml (vendor preset)` is:

| Tool | Fragment |
|------|----------|
| VitePress (the live docs site) | `#config-base-yaml-vendor-preset` |
| lychee `--include-fragments` | `#configbaseyaml-vendor-preset` |

Both guesses look plausible. Review and fix agents flip-flopped between them
across multiple rounds on a docs PR, even though the guide content was
correct. The docs site is built by VitePress, so VitePress is authoritative.

## Which tool is authoritative

| Check | Hook | What it validates |
|-------|------|-------------------|
| Linked file exists | `lint-md-links` (lychee, **without** `--include-fragments`) | The path in a markdown link resolves on disk |
| `#fragment` matches a heading | `lint-docs-anchors` | The fragment is a VitePress heading id, a `{#explicit-id}` attr, or an HTML `id=` in the target `docs/` page |

`hack/lint-docs-anchors` reimplements the `@mdit-vue/shared` slugify function
used by the pinned VitePress version in `package.json`. If the two ever
diverge after a VitePress upgrade, update the reimplementation and its tests
in `hack/lint-docs-anchors-test.py`.

Do **not** change a fragment to match lychee when `lint-docs-anchors` rejects
it. The hook prints the VitePress id to use.

## Writing heading links

1. Use the slug VitePress generates. Punctuation becomes a hyphen, and
   consecutive punctuation collapses to one hyphen:
   `config.base.yaml (vendor preset)` → `config-base-yaml-vendor-preset`.
2. Headings that start with a digit get a `_` prefix:
   `1. Webhook + dispatch service` → `_1-webhook-dispatch-service`.
3. To pin an id (for example when a heading is likely to be renamed, or when
   you want a stable target that is not the generated slug), add
   `{#my-stable-id}` at the end of the heading or an HTML
   `<a id="my-stable-id"></a>` immediately before it.

## Running locally

```bash
./hack/lint-docs-anchors
# or, file links + fragments together:
make lint-md-links
```

The pre-commit hook `lint-docs-anchors` scans the whole `docs/` tree whenever
any `docs/**/*.md` file is staged, so a heading rename is still caught if
another page still points at the old slug. `make lint-all` does the same.
