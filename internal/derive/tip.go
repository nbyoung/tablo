package derive

// AtTip returns the status of the task a snapshot reads as the files stand
// at the tip of the trunk of the project it reads: its gate and its state,
// and no fact of the history. It stands beside Pin.Behind, and tells a pin
// whose advance changes what the project in view sees from one that is
// merely behind (decision 12). It is nil where the tip or the task is
// undetermined: for a nil snapshot, a link with no project, a project with
// no history or no trunk, and a tip that holds no such task.
func (f *Facts) AtTip(n *Snapshot) *Status {
	if n == nil || n.Facts == nil || !f.enter() {
		return nil
	}
	defer f.leave()
	sub := n.Facts
	if sub.view != nil {
		sub = sub.view
	}
	if sub.log == nil || sub.log.Trunk.Tip == "" {
		return nil
	}
	tip := sub.past(sub.log.Trunk.Tip)
	if tip == nil || tip.refused != nil {
		return nil
	}
	return tip.status(n.Target)
}
