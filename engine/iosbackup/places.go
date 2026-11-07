package iosbackup

import "strings"

// place is the folder a domain's files are shown in.
func place(domain string) string {
	switch {
	case domain == "CameraRollDomain":
		return "Camera Roll"
	case domain == "MediaDomain":
		return "Media"
	case domain == "HomeDomain":
		return "Phone"
	case strings.HasPrefix(domain, "AppDomainGroup-"):
		return "Apps/Shared/" + strings.TrimPrefix(domain, "AppDomainGroup-")
	case strings.HasPrefix(domain, "AppDomainPlugin-"):
		return "Apps/Extensions/" + strings.TrimPrefix(domain, "AppDomainPlugin-")
	case strings.HasPrefix(domain, "AppDomain-"):
		return "Apps/" + strings.TrimPrefix(domain, "AppDomain-")
	}
	return "System/" + domain
}

// shorten drops the "Media/" every Camera Roll path starts with.
func shorten(domain, rel string) string {
	if domain == "CameraRollDomain" {
		return strings.TrimPrefix(rel, "Media/")
	}
	return rel
}
