package flowbuilder

/*
func (ge *FlowEngine) loadExternalFlows(ctx *base.Context) {
	ge.flowLock.Lock()
	defer ge.flowLock.Unlock()

	config := ge.ReadConfig()
	var err error

	for _, fURL := range config.FlowsFromURL {
		newFlows := make(map[string]FlowDesc)
		err = ge.cr.ReadObjectFromURL(&newFlows, fURL)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fURL, err)
		}
		ge.addExternalFlowsWithSource(ctx, newFlows, "url: "+fURL)
	}
	config.FlowsFromURL = []string{}
	for _, fName := range config.FlowsFromFile {
		newFlows := make(map[string]FlowDesc)
		err = ge.cr.ReadObjectFromFile(&newFlows, fName)
		if err != nil {
			log.Errorf("Skipping %v, because: %v", fName, err)
		}
		ge.addExternalFlowsWithSource(ctx, newFlows, "file: "+fName)
	}
	config.FlowsFromFile = []string{}
	err = ge.cr.WriteSection("flows", &config, true)
	if err != nil {

	}
}

func (ge *FlowEngine) addExternalFlowsWithSource(ctx *base.Context, src map[string]FlowDesc, srcName string) {
	for k, v := range src {
		v.Source = srcName
		err := ge.addFlowUnderLock(ctx, k, v, true, true)
		if err != nil {
			ctx.GetLogger().Errorf("Skipping flow %v from %v, because: %v", k, srcName, err)
		}
	}
}
*/
