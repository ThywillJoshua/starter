package convert

import "strings"

// buildHierarchy turns a flat, depth-annotated section list into a tree.
func buildHierarchy(sections []Section) []Section {
    // Keep a stack of the last seen section at each depth.
    var roots []Section
    var stack []int // indices in result slice (flattened order not used after)
    // We'll build into a new slice to avoid mutating input references unexpectedly.
    var out []Section

    // helper to append child to parent referenced by index in out
    appendChild := func(parentIdx int, child Section) {
        p := out[parentIdx]
        p.Children = append(p.Children, child)
        out[parentIdx] = p
    }

    for _, s := range sections {
        // trim stack to parent depth (depth-1)
        for len(stack) >= s.Depth {
            stack = stack[:len(stack)-1]
        }
        if len(stack) == 0 {
            // top-level
            roots = append(roots, s)
            out = append(out, s)
            stack = append(stack, len(out)-1)
            continue
        }
        // find current parent index
        parentIdx := stack[len(stack)-1]
        // sanity: ensure numbering hierarchy matches prefix (e.g., 2.1 under 2)
        parent := out[parentIdx]
        if strings.HasPrefix(s.Number+".", parent.Number+".") || strings.HasPrefix(s.Number, parent.Number+".") {
            appendChild(parentIdx, s)
            // push this as potential parent for deeper children
            // We need to address it by index inside parent's Children; since out is value-copied, we can't index nested child directly.
            // Instead, push a pseudo index by appending s to out as well for stack tracking.
            out = append(out, s)
            stack = append(stack, len(out)-1)
        } else {
            // if numbering doesn't match, treat as new root-level to avoid losing section
            roots = append(roots, s)
            out = append(out, s)
            stack = []int{len(out) - 1}
        }
    }
    // The 'roots' slice contains only top-level Sections without populated Children; rebuild it from 'out'
    // Build a map from number to section (with children)
    // For simplicity, reconstruct by scanning 'out' and connecting by depth/number again
    return attachChildren(sections)
}

func attachChildren(flat []Section) []Section {
    // map number -> pointer index in array
    nodes := make([]Section, len(flat))
    copy(nodes, flat)
    var roots []Section
    for i := range nodes {
        s := nodes[i]
        // find closest parent: scan backwards for a section with lower depth and prefix match
        parentIdx := -1
        for j := i - 1; j >= 0; j-- {
            if nodes[j].Depth < s.Depth && strings.HasPrefix(s.Number, nodes[j].Number+".") {
                parentIdx = j
                break
            }
        }
        if parentIdx == -1 {
            roots = append(roots, nodes[i])
            continue
        }
        p := nodes[parentIdx]
        p.Children = append(p.Children, nodes[i])
        nodes[parentIdx] = p
    }
    return rebuildWithChildrenOrder(roots, nodes)
}

func rebuildWithChildrenOrder(roots []Section, nodes []Section) []Section {
    // nodes contains children attached but roots entries are old copies; refresh them by matching numbers
    byNumber := map[string]Section{}
    for _, n := range nodes {
        byNumber[n.Number] = n
    }
    out := make([]Section, 0, len(roots))
    for _, r := range roots {
        if v, ok := byNumber[r.Number]; ok {
            out = append(out, v)
        } else {
            out = append(out, r)
        }
    }
    return out
}

