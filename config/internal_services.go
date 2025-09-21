package config

// InternalServices contains all internal service URLs.
var InternalServices = map[string]string{
	"segmentationService": "http://segmentation-service.segmentation-service.svc.cluster.local",
	"analyticsService": "http://analytics-service.analytics-service.svc.cluster.local",
	"authService": "http://new-auth-service.new-auth-service.svc.cluster.local",
	"blockChainHookService": "http://blockchain-request-hook-service.blockchain-request-hook-service.svc.cluster.local",
	"blockChainRequestService": "http://blockchain-request-service.blockchain-request-service.svc.cluster.local",
	"blockChainService": "http://flow-service.flow-service.svc.cluster.local",
	"campaignService": "http://campaign-service.campaign-service.svc.cluster.local",
	"controlService": "http://control-service.control-service.svc.cluster.local",
	"entityService": "https://rest.entitysport.com/v2",
	"eventService": "http://event-service.event-service.svc.cluster.local",
	"faqService": "http://faq-service.faq-service.svc.cluster.local/faq",
	"fantasyService": "http://fantasy-service.fantasy-service.svc.cluster.local",
	"fc2Service": "http://challenge-service.challenge-service.svc.cluster.local/V2/challenge",
	"fc3Service": "http://challenge-service.challenge-service.svc.cluster.local/V3/challenge",
	"fcService": "http://challenge-service.challenge-service.svc.cluster.local/challenge",
	"financeService": "http://finance-service.finance-service.svc.cluster.local",
	"ipfsService": "https://ipfs-master-jzqvjpxgsa-el.a.run.app",
	"kycService": "http://kyc-service.kyc-service.svc.cluster.local",
	"leaderboardService": "http://leaderboard-service.leaderboard-service.svc.cluster.local",
	"momentLeaderboardService": "http://moment-leaderboard-service.moment-leaderboard-service.svc.cluster.local",
	"milestoneService": "http://milestone-service.milestone-service.svc.cluster.local",
	"miniGamesService": "http://mini-games-service.mini-games-service.svc.cluster.local",
	"mintFactoryService": "http://mint-factory-service.mint-factory-service.svc.cluster.local",
	"momentMarketPlaceService": "http://moment-mp-service.moment-service.svc.cluster.local/moment",
	"momentMintingService": "http://moment-minting-service.moment-service.svc.cluster.local/moment",
	"momentPoolService": "http://moment-pool-service.moment-service.svc.cluster.local/moment",
	"lockingService": "http://locking-service.locking-service.svc.cluster.local",
	"nbaChallengeService": "http://nba-flash-service.nba-flash-service.svc.cluster.local/nbaChallenge",
	"nbaDataService": "http://nba-data-service.nba-data-service.svc.cluster.local/nbaData",
	"nftService": "http://nft-service.nft-service.svc.cluster.local",
	"notificationService": "http://notification-service.notification-service.svc.cluster.local",
	"notificationV2Service": "http://notification-service-v2.notification-service-v2.svc.cluster.local/notify/v2",
	"packService": "http://packs-service.packs-service.svc.cluster.local/packs",
	"partnerService": "http://partner-service.partner-service.svc.cluster.local/partners",
	"personalizationService": "http://personalization-service.personalization-service.svc.cluster.local",
	"playerService": "http://team-service.team-service.svc.cluster.local/player",
	"profileService": "http://profile-service.profile-service.svc.cluster.local/profile",
	"rewardService": "http://reward-service.reward-service.svc.cluster.local",
	"riskAnalysisService": "http://risk-analysis-service.risk-analysis-service.svc.cluster.local",
	"schedulerService": "http://cron-scheduler-service.cron-scheduler-service.svc.cluster.local",
	"scrapeService": "https://python-scripts-jzqvjpxgsa-el.a.run.app",
	"setService": "http://set-service.set-service.svc.cluster.local/set",
	"simulationService": "http://simulation-service.simulation-service.svc.cluster.local",
	"sportsService": "http://sports-service.sports-service.svc.cluster.local",
	"teamService": "http://team-service.team-service.svc.cluster.local/team",
	"userService": "http://auth-service.auth-service.svc.cluster.local/users",
	"walletService": "http://wallet-service.wallet-service.svc.cluster.local",
	"freshdeskService": "http://freshdesk-service.freshdesk-service.svc.cluster.local",
}

// ProxyServices generates the proxy URLs if needed
func ProxyServices(stage string) map[string]string {
	proxies := make(map[string]string)
	for k := range InternalServices {
		proxies[k] = "https://proxy." + stage + ".munna-bhai.xyz/proxy/" + k
	}
	return proxies
}
