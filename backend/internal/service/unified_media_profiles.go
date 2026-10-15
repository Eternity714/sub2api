package service

func mediaAdapterProfile(platform, mediaType, model string) UnifiedMediaAdapterProfile {
	loadMediaCapabilityCatalog()
	return copyMediaAdapterProfile(mediaCatalogProfiles[platform+":"+mediaType+":"+model])
}
