# README Diagram Refresh

README diagram refreshes should treat rendered PNGs as the acceptance artifact,
not just SVG source.

The useful closure pattern is:

- reread the README and source package before redrawing a module diagram;
- keep README assets in `docs/images/readme-diagrams` as paired `.svg` and
  `.png` files;
- remove obsolete Graphviz scripts and intermediate `.dot`/`.plain` assets once
  the final README diagrams are source-owned SVGs;
- use only `Architects Daughter` and `Comic Mono` in SVG styles so renderer
  fallback does not silently change text width;
- render with CairoSVG, then inspect the PNG at full size for text that touches
  card boundaries, labels sitting on unrelated arrows, and sequence `alt`
  regions that hide or cross lifelines;
- create a contact sheet for the full diagram set before final commit, then
  reopen dense diagrams individually.

For future passes, avoid broad batch acceptance after a script succeeds. The
failure mode here was subtle: SVG XML and README links were valid, but rendered
PNG inspection still found card text overflow and an `alt` note overlapping
sequence lifelines.
