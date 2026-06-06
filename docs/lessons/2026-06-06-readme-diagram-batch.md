# README Diagram Batch

Issues #133 and #134 are documentation coverage gates, not API implementation
issues.

The useful closure pattern is:

- reuse existing accepted diagrams when they already satisfy part of the gate;
- generate `.dot`, `.plain`, `-graphviz.svg`, `-graphviz.png`, final `.svg`,
  and final `.png` together so evidence and README assets do not drift;
- embed only PNG files from README files, with adjacent SVG sources kept for
  inspection and future edits;
- inspect a rendered PNG contact sheet before committing, because Graphviz edge
  labels can overlap even when layout generation succeeds.

For future milestones, keep edge meaning in node text when orthogonal README
diagrams get crowded. Unlabeled connectors are usually more readable than
source-grounded labels that collide with nodes.
