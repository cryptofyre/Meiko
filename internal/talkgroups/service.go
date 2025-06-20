package talkgroups

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"Meiko/internal/config"
	"Meiko/internal/logger"
)

// ServiceType represents different emergency service categories
type ServiceType string

const (
	ServicePolice      ServiceType = "POLICE"
	ServiceFire        ServiceType = "FIRE"
	ServiceEMS         ServiceType = "EMS"
	ServiceEmergency   ServiceType = "EMERGENCY"
	ServicePublicWorks ServiceType = "PUBLIC_WORKS"
	ServiceEducation   ServiceType = "EDUCATION"
	ServiceEvents      ServiceType = "EVENTS"
	ServiceAirport     ServiceType = "AIRPORT"
	ServiceOther       ServiceType = "OTHER"
)

// TalkgroupInfo contains enhanced talkgroup information
type TalkgroupInfo struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Group       string      `json:"group"`
	Color       string      `json:"color"`
	ServiceType ServiceType `json:"service_type"`
	Emoji       string      `json:"emoji"`
	ColorHex    string      `json:"color_hex"`
}

// DepartmentType contains department classification information
type DepartmentType struct {
	Keywords []string    `json:"keywords"`
	Color    string      `json:"color"`
	Emoji    string      `json:"emoji"`
	Type     ServiceType `json:"type"`
}

// Playlist represents the SDRTrunk playlist XML structure
type Playlist struct {
	XMLName xml.Name `xml:"playlist"`
	Version string   `xml:"version,attr"`
	Aliases []Alias  `xml:"alias"`
}

// Alias represents a single talkgroup alias in the playlist
type Alias struct {
	XMLName xml.Name `xml:"alias"`
	Group   string   `xml:"group,attr"`
	Color   string   `xml:"color,attr"`
	Name    string   `xml:"name,attr"`
	List    string   `xml:"list,attr"`
	IDs     []ID     `xml:"id"`
}

// ID represents an alias ID element
type ID struct {
	XMLName  xml.Name `xml:"id"`
	Type     string   `xml:"type,attr"`
	Value    string   `xml:"value,attr"`
	Protocol string   `xml:"protocol,attr"`
	Channel  string   `xml:"channel,attr"`
	Priority string   `xml:"priority,attr"`
}

// Service handles talkgroup information and categorization
type Service struct {
	talkgroups      map[string]*TalkgroupInfo
	departmentTypes map[ServiceType]*DepartmentType
	config          *config.Config
	logger          *logger.Logger
	lastLoaded      time.Time
}

// New creates a new talkgroup service
func New(config *config.Config, logger *logger.Logger) *Service {
	if config == nil {
		panic("config cannot be nil")
	}
	if logger == nil {
		panic("logger cannot be nil")
	}

	service := &Service{
		talkgroups: make(map[string]*TalkgroupInfo),
		config:     config,
		logger:     logger,
	}

	service.initDepartmentTypes()

	// Load talkgroups if playlist path is configured
	if config.Talkgroups.PlaylistPath != "" {
		if err := service.LoadPlaylist(config.Talkgroups.PlaylistPath); err != nil {
			logger.Warn("Failed to load talkgroup playlist", "error", err, "path", config.Talkgroups.PlaylistPath)
		}
	}

	return service
}

// initDepartmentTypes sets up the department classification system
func (s *Service) initDepartmentTypes() {
	s.departmentTypes = map[ServiceType]*DepartmentType{
		ServicePolice: {
			Keywords: []string{
				"PD", "Police", "Sheriff", "SO", "Law", "Enforcement", "MCSO", "Constb",
				"TSTC Police", "Baylor PD", "Patrol", "Disp", "CID", "SpOp", "Ops", "AllCal",
				"Woodway Police", "WPD", "Deputy", "Detective", "K9", "SWAT", "Tactical",
				"McLennan", "Robinson", "Hewitt", "Lorena", "Bruceville", "Eddy", "Mart",
				"Moody", "McGregor", "Crawford", "Elm Mott", "Lacy", "Riesel", "Valley Mills",
			},
			Color: "#0037ff",
			Emoji: "👮",
			Type:  ServicePolice,
		},
		ServiceFire: {
			Keywords: []string{
				"FD", "Fire", "WFD", "Still Cl", "Tone", " FD ", "Disp FD", "FD Disp", " Fire ", "Fire Dept",
				"Engine", "Ladder", "Truck", "Rescue", "Chief", "Battalion", "Squad", "Hazmat",
				"Woodway Fire", "McLennan Fire", "Robinson Fire", "Hewitt Fire", "Bellmead Fire",
			},
			Color: "#ff0000",
			Emoji: "🚒",
			Type:  ServiceFire,
		},
		ServiceEMS: {
			Keywords: []string{
				"EMS", "Medical", "Ambulance", "Medic", "Rescue", "Paramedic", "EMT",
				"MedStar", "AMR", "MCHD", "Life Flight", "Air Evac", "Mercy", "Emergency Medical",
			},
			Color: "#00aa00",
			Emoji: "🚑",
			Type:  ServiceEMS,
		},
		ServiceEmergency: {
			Keywords: []string{"Emer", "EOC", "Emergency", "T-Control", "Mgmt"},
			Color:    "#ff7700",
			Emoji:    "🚨",
			Type:     ServiceEmergency,
		},
		ServicePublicWorks: {
			Keywords: []string{"PW", "Public Works", "Streets", "Util", "Park", "Fleet", "Traffic", "Garbg", "Garb", "Roads", "Sewer", "Water", "Meter", "Wtr", "Strt", "Traff", "Bldg"},
			Color:    "#2db82d",
			Emoji:    "🔧",
			Type:     ServicePublicWorks,
		},
		ServiceEducation: {
			Keywords: []string{"ISD", "School", "WISD", "CISD", "Campus", "MCC", "HS", "University", "College"},
			Color:    "#9933ff",
			Emoji:    "🎓",
			Type:     ServiceEducation,
		},
		ServiceEvents: {
			Keywords: []string{"Events", "RadioSvc", "Radio"},
			Color:    "#ffcc00",
			Emoji:    "📡",
			Type:     ServiceEvents,
		},
		ServiceAirport: {
			Keywords: []string{"Airprt", "Airport"},
			Color:    "#00ccff",
			Emoji:    "✈️",
			Type:     ServiceAirport,
		},
		ServiceOther: {
			Keywords: []string{},
			Color:    "#0099ff",
			Emoji:    "🔔",
			Type:     ServiceOther,
		},
	}
}

// LoadPlaylist loads talkgroup information from an SDRTrunk playlist XML file
func (s *Service) LoadPlaylist(filePath string) error {
	if s.departmentTypes == nil {
		s.initDepartmentTypes()
	}

	s.logger.Info("Loading talkgroup playlist", "path", filePath)

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read playlist file: %w", err)
	}

	var playlist Playlist
	if err := xml.Unmarshal(data, &playlist); err != nil {
		return fmt.Errorf("failed to parse playlist XML: %w", err)
	}

	count := 0
	for _, alias := range playlist.Aliases {
		// Find talkgroup ID
		var talkgroupID string
		for _, id := range alias.IDs {
			if id.Type == "talkgroup" && id.Value != "" {
				talkgroupID = id.Value
				break
			}
		}

		if talkgroupID != "" {
			serviceType := s.classifyDepartment(alias.Group, alias.Name)
			deptInfo, exists := s.departmentTypes[serviceType]
			if !exists {
				// Fallback to ServiceOther if department type not found
				serviceType = ServiceOther
				deptInfo = s.departmentTypes[ServiceOther]
			}

			talkgroupInfo := &TalkgroupInfo{
				ID:          talkgroupID,
				Name:        alias.Name,
				Group:       alias.Group,
				Color:       alias.Color,
				ServiceType: serviceType,
				Emoji:       deptInfo.Emoji,
				ColorHex:    deptInfo.Color,
			}

			s.talkgroups[talkgroupID] = talkgroupInfo
			count++
		}
	}

	s.lastLoaded = time.Now()
	s.logger.Success("Loaded talkgroup playlist", "count", count, "file", filepath.Base(filePath))

	// Log department breakdown
	serviceCounts := make(map[ServiceType]int)
	for _, tg := range s.talkgroups {
		serviceCounts[tg.ServiceType]++
	}

	s.logger.Info("Department breakdown",
		"police", serviceCounts[ServicePolice],
		"fire", serviceCounts[ServiceFire],
		"ems", serviceCounts[ServiceEMS],
		"emergency", serviceCounts[ServiceEmergency],
		"public_works", serviceCounts[ServicePublicWorks],
		"education", serviceCounts[ServiceEducation],
		"events", serviceCounts[ServiceEvents],
		"airport", serviceCounts[ServiceAirport],
		"other", serviceCounts[ServiceOther])

	return nil
}

// classifyDepartment determines the service type based on group and name
func (s *Service) classifyDepartment(group, name string) ServiceType {
	combined := strings.ToUpper(fmt.Sprintf("%s %s", group, name))

	// Check each department type for keyword matches
	for serviceType, dept := range s.departmentTypes {
		for _, keyword := range dept.Keywords {
			if strings.Contains(combined, strings.ToUpper(keyword)) {
				s.logger.Debug("Talkgroup classified",
					"group", group,
					"name", name,
					"combined", combined,
					"matched_keyword", keyword,
					"service_type", string(serviceType))
				return serviceType
			}
		}
	}

	// Log unclassified talkgroups to help with troubleshooting
	s.logger.Debug("Talkgroup unclassified",
		"group", group,
		"name", name,
		"combined", combined,
		"defaulting_to", "OTHER")

	return ServiceOther
}

// GetTalkgroupInfo returns enhanced talkgroup information
func (s *Service) GetTalkgroupInfo(talkgroupID string) *TalkgroupInfo {
	if info, exists := s.talkgroups[talkgroupID]; exists {
		return info
	}

	// Return default info for unknown talkgroups
	return &TalkgroupInfo{
		ID:          talkgroupID,
		Name:        fmt.Sprintf("TG %s", talkgroupID),
		Group:       "Unknown Department",
		Color:       "0",
		ServiceType: ServiceOther,
		Emoji:       "🔔",
		ColorHex:    "#0099ff",
	}
}

// GetTalkgroupInfoWithContext returns enhanced talkgroup information with intelligent classification
// based on call context (who is calling whom). If the caller is unknown but the called party
// is a known department, it will assume the caller is from the same department type.
func (s *Service) GetTalkgroupInfoWithContext(talkgroupID, contextTalkgroupID string) *TalkgroupInfo {
	// If we have direct information about this talkgroup, use it
	if info, exists := s.talkgroups[talkgroupID]; exists {
		return info
	}

	// If we don't have context, fall back to default behavior
	if contextTalkgroupID == "" || contextTalkgroupID == talkgroupID {
		return s.GetTalkgroupInfo(talkgroupID)
	}

	// Get information about the context talkgroup (the one being called)
	contextInfo := s.GetTalkgroupInfo(contextTalkgroupID)

	// If the context talkgroup is also unknown, can't infer anything
	if contextInfo.ServiceType == ServiceOther {
		return s.GetTalkgroupInfo(talkgroupID)
	}

	// Get department information for the context talkgroup
	contextDept, exists := s.departmentTypes[contextInfo.ServiceType]
	if !exists {
		return s.GetTalkgroupInfo(talkgroupID)
	}

	// Create enhanced info by inheriting from the context department
	enhancedInfo := &TalkgroupInfo{
		ID:          talkgroupID,
		Name:        fmt.Sprintf("TG %s", talkgroupID), // Keep the TG format for unknown talkgroups
		Group:       contextInfo.Group,                 // Inherit the department group
		Color:       contextInfo.Color,                 // Inherit color
		ServiceType: contextInfo.ServiceType,           // Inherit service type
		Emoji:       contextDept.Emoji,                 // Use department emoji
		ColorHex:    contextDept.Color,                 // Use department color
	}

	s.logger.Debug("Talkgroup classified via context",
		"talkgroup_id", talkgroupID,
		"context_talkgroup_id", contextTalkgroupID,
		"inferred_service_type", string(enhancedInfo.ServiceType),
		"inferred_group", enhancedInfo.Group)

	return enhancedInfo
}

// GetDepartmentInfo returns department classification information
func (s *Service) GetDepartmentInfo(talkgroupID string) *DepartmentType {
	info := s.GetTalkgroupInfo(talkgroupID)
	if dept, exists := s.departmentTypes[info.ServiceType]; exists {
		return dept
	}

	// Return default for unknown departments
	return &DepartmentType{
		Keywords: []string{},
		Color:    "#0099ff",
		Emoji:    "🔔",
		Type:     ServiceOther,
	}
}

// GetDepartmentInfoWithContext returns department classification information with context awareness
func (s *Service) GetDepartmentInfoWithContext(talkgroupID, contextTalkgroupID string) *DepartmentType {
	info := s.GetTalkgroupInfoWithContext(talkgroupID, contextTalkgroupID)
	if dept, exists := s.departmentTypes[info.ServiceType]; exists {
		return dept
	}

	// Return default for unknown departments
	return &DepartmentType{
		Keywords: []string{},
		Color:    "#0099ff",
		Emoji:    "🔔",
		Type:     ServiceOther,
	}
}

// FormatTalkgroupDisplay creates a formatted display string for talkgroups
func (s *Service) FormatTalkgroupDisplay(talkgroupID string) string {
	info := s.GetTalkgroupInfo(talkgroupID)

	// Create formatted display with emoji and department
	if info.ServiceType != ServiceOther {
		return fmt.Sprintf("%s %s", info.Emoji, info.Name)
	}

	return fmt.Sprintf("TG %s", talkgroupID)
}

// FormatTalkgroupDisplayWithContext creates a formatted display string for talkgroups with context awareness
func (s *Service) FormatTalkgroupDisplayWithContext(talkgroupID, contextTalkgroupID string) string {
	info := s.GetTalkgroupInfoWithContext(talkgroupID, contextTalkgroupID)

	// Create formatted display with emoji and department
	if info.ServiceType != ServiceOther {
		return fmt.Sprintf("%s %s", info.Emoji, info.Name)
	}

	return fmt.Sprintf("TG %s", talkgroupID)
}

// GetAllTalkgroups returns all loaded talkgroups
func (s *Service) GetAllTalkgroups() map[string]*TalkgroupInfo {
	return s.talkgroups
}

// GetServiceTypes returns all available service types
func (s *Service) GetServiceTypes() map[ServiceType]*DepartmentType {
	return s.departmentTypes
}

// GetStats returns talkgroup service statistics
func (s *Service) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})
	stats["total_talkgroups"] = len(s.talkgroups)
	stats["last_loaded"] = s.lastLoaded

	// Count by service type
	serviceTypeCounts := make(map[string]int)
	for _, tg := range s.talkgroups {
		serviceTypeCounts[string(tg.ServiceType)]++
	}
	stats["by_service_type"] = serviceTypeCounts

	return stats
}

// ReloadPlaylist reloads the playlist file
func (s *Service) ReloadPlaylist() error {
	if s.config.Talkgroups.PlaylistPath == "" {
		return fmt.Errorf("no playlist path configured")
	}

	return s.LoadPlaylist(s.config.Talkgroups.PlaylistPath)
}
