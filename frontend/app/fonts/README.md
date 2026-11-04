# Web font subsets

## Handwriting

`mtv2-handwriting-subset.woff2` is a web-only subset generated from
LXGW WenKai Lite Regular v1.522. The upstream source is distributed under
the SIL Open Font License 1.1; the complete license is included in `OFL.txt`.

Upstream source:
https://github.com/lxgw/LxgwWenKai-Lite/releases/tag/v1.522

Source SHA-256:
`140c99ba4e28e817cec49bf82a0c5fcdc4fe633fb9dfda16d0ee8d59a8545f15`

The subset contains only the characters used by `.search-scribble`,
`.search-footer-note`, and `.auth-kicker`. If those strings change, regenerate
the subset from the licensed source and update this note.

## Noto Sans SC / Noto Serif SC

`noto-sans-sc-ui.woff2` and `noto-serif-sc-ui.woff2` are web-only variable-font
subsets generated from the Noto CJK SC variable fonts. They are built into the
app so `next build` does not depend on fonts.googleapis.com.

Upstream sources:

- https://github.com/notofonts/noto-cjk/raw/main/Sans/Variable/TTF/Subset/NotoSansSC-VF.ttf
- https://github.com/notofonts/noto-cjk/raw/main/Serif/Variable/TTF/Subset/NotoSerifSC-VF.ttf

Copyright notices from the fonts:

- Noto Sans SC: © 2014-2021 Adobe (http://www.adobe.com/), with Reserved Font Name 'Source'.
- Noto Serif SC: © 2017-2024 Adobe (http://www.adobe.com/).

Both are licensed under the SIL Open Font License 1.1; see `OFL-Noto.txt`.

The subsets contain the characters used by the frontend source. Characters
outside the subset (for example rare glyphs in user comments) fall back to the
next font in the CSS stack. Regenerate after adding UI text:

```bash
pyftsubset NotoSansSC-VF.ttf --text-file=ui-chars.txt --flavor=woff2 \
  --output-file=noto-sans-sc-ui.woff2 --layout-features='*' --no-hinting
pyftsubset NotoSerifSC-VF.ttf --text-file=ui-chars.txt --flavor=woff2 \
  --output-file=noto-serif-sc-ui.woff2 --layout-features='*' --no-hinting
```
