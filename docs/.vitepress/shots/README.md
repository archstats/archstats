# Screenshots for the docs

The pictures in `docs/public/shots` are Archstats Desktop on open-source codebases. Each comes as `<name>.webp` and `<name>-dark.webp`, and the `Shot` component shows the one that matches the page's appearance.

| Shot | Codebase | View |
|---|---|---|
| `overview` | Apache Fineract, `develop` @ c516013 | `/` (full window, 1760×1100) |
| `units` | Apache Fineract | `/views/units` |
| `cycles` | Apache Fineract | `/views/components/cycles` |
| `hotspots` | Apache Fineract | `/views/components/hotspots` |
| `http` | Apache Fineract | `/views/http` |
| `metrics` | Apache Fineract | `/views/metrics?view=plot&preset=dms` |
| `activity` | Apache Fineract | `/views/git/activity` |
| `cicd` | LibreChat, `main` @ be4cf58 | `/views/cicd` |

All but `overview` are cropped to the view (the app's sidebar left out) and saved at 1600×1200.

Use public codebases only. A workspace scanned from client or employer code never goes on this site, and neither do people's names: leave the Authors views out.

## Taking them again

1. Run `wails dev` in archstats-ui, with the workspaces you want scanned in the app.
2. Build the committed frontend somewhere else, so the shots show a released UI and not work in progress:

    ```shell
    mkdir -p /tmp/ui && git -C ../archstats-ui archive HEAD frontend | tar -x -C /tmp/ui
    ln -s "$PWD/../archstats-ui/frontend/node_modules" /tmp/ui/frontend/node_modules
    (cd /tmp/ui/frontend && NUXT_DIST_OUTPUT=1 npx nuxt generate)
    ```

3. Capture at twice the size, in both appearances:

    ```shell
    node shoot.mjs --dist /tmp/ui/frontend/dist --workspace fineract --out /tmp/shots \
      --scheme both --dpr 2 --wait 8000 \
      --routes "overview=/,units=/views/units,cycles=/views/components/cycles"
    ```

    A view that is still loading shows "Reading …"; raise `--wait` and take it again.

4. Crop and convert: keep `overview` whole and scale it to 1760×1100; crop the 480px sidebar (at 2×) off the others and scale them to 1600×1200. Save as WebP at quality 84 into `docs/public/shots`.
