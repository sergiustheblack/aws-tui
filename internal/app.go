package internal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	aws "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/acmpca"
	cf "github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cw "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwLogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ec "github.com/aws/aws-sdk-go-v2/service/elasticache"
	elb "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	ga "github.com/aws/aws-sdk-go-v2/service/globalaccelerator"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	msk "github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/mq"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	r53 "github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	sq "github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/bporter816/aws-tui/internal/repo"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Application struct {
	app         *tview.Application
	pages       *tview.Pages
	layout      *tview.Flex
	header      *Header
	footer      *Footer
	filterInput *tview.InputField
	components  []Component
	region      string
	profile     string
	ctx         context.Context
	cancel      context.CancelFunc
}

func buildRepos(cfg aws.Config) (Repos, *repo.STS) {
	// Global Accelerator requires us-west-2 regardless of selected region.
	westCfg := cfg.Copy()
	westCfg.Region = "us-west-2"

	httpClient := &http.Client{}

	acmClient := acm.NewFromConfig(cfg)
	acmPCAClient := acmpca.NewFromConfig(cfg)
	cfClient := cf.NewFromConfig(cfg)
	cwClient := cw.NewFromConfig(cfg)
	cwLogsClient := cwLogs.NewFromConfig(cfg)
	ddbClient := ddb.NewFromConfig(cfg)
	ecClient := ec.NewFromConfig(cfg)
	ec2Client := ec2.NewFromConfig(cfg)
	ecsClient := ecs.NewFromConfig(cfg)
	eksClient := eks.NewFromConfig(cfg)
	elbClient := elb.NewFromConfig(cfg)
	gaClient := ga.NewFromConfig(westCfg)
	iamClient := iam.NewFromConfig(cfg)
	kmsClient := kms.NewFromConfig(cfg)
	lambdaClient := lambda.NewFromConfig(cfg)
	mqClient := mq.NewFromConfig(cfg)
	mskClient := msk.NewFromConfig(cfg)
	rdsClient := rds.NewFromConfig(cfg)
	r53Client := r53.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)
	smClient := sm.NewFromConfig(cfg)
	sqClient := sq.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)
	ssmClient := ssm.NewFromConfig(cfg)
	stsClient := sts.NewFromConfig(cfg)

	repos := Repos{
		ACM:               repo.NewACM(acmClient),
		ACMPCA:            repo.NewACMPCA(acmPCAClient),
		CloudFront:        repo.NewCloudFront(cfClient),
		CloudWatch:        repo.NewCloudWatch(cwLogsClient),
		DynamoDB:          repo.NewDynamoDB(ddbClient),
		EC2:               repo.NewEC2(ec2Client),
		ECS:               repo.NewECS(ecsClient),
		EKS:               repo.NewEKS(eksClient),
		ELB:               repo.NewELB(elbClient, httpClient),
		ElastiCache:       repo.NewElastiCache(ecClient, cwClient),
		GlobalAccelerator: repo.NewGlobalAccelerator(gaClient),
		IAM:               repo.NewIAM(iamClient),
		KMS:               repo.NewKMS(kmsClient),
		Lambda:            repo.NewLambda(lambdaClient),
		MQ:                repo.NewMQ(mqClient),
		MSK:               repo.NewMSK(mskClient),
		RDS:               repo.NewRDS(rdsClient),
		Route53:           repo.NewRoute53(r53Client),
		S3:                repo.NewS3(s3Client),
		SecretsManager:    repo.NewSecretsManager(smClient),
		ServiceQuotas:     repo.NewServiceQuotas(sqClient),
		SNS:               repo.NewSNS(snsClient),
		SQS:               repo.NewSQS(sqsClient),
		SSM:               repo.NewSSM(ssmClient),
	}
	stsRepo := repo.NewSTS(stsClient)
	return repos, stsRepo
}

// defaultRegion is used when neither the environment nor the profile in the
// shared config file sets a region.
const defaultRegion = "us-east-1"

// loadConfig loads the AWS config the same way the AWS CLI does, honoring
// AWS_PROFILE, AWS_REGION, AWS_CONFIG_FILE etc. A non-empty profile or region
// overrides the environment, like --profile and --region do.
func loadConfig(profile, region string) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithDefaultRegion(defaultRegion),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	return config.LoadDefaultConfig(context.TODO(), opts...)
}

// envProfile returns the profile selected by the environment, if any.
func envProfile() string {
	if p := os.Getenv("AWS_PROFILE"); p != "" {
		return p
	}
	return os.Getenv("AWS_DEFAULT_PROFILE")
}

func NewApplication() *Application {
	cfg, err := loadConfig("", "")
	if err != nil {
		panic(err)
	}

	repos, stsRepo := buildRepos(cfg)

	a := &Application{
		region:  cfg.Region,
		profile: envProfile(),
	}
	a.ctx, a.cancel = context.WithCancel(context.Background())

	tviewApp := tview.NewApplication()
	pages := tview.NewPages()
	pages.SetBorder(true)

	header := NewHeader(stsRepo, repos.IAM, a)
	footer := NewFooter(a)

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(header, 4, 0, false) // header is 4 rows
	flex.AddItem(pages, 0, 1, true)   // main viewport is resizable
	flex.AddItem(footer, 1, 0, false) // footer is 1 row

	tviewApp.SetRoot(flex, true).SetFocus(pages)
	a.app = tviewApp
	a.pages = pages
	a.layout = flex
	a.header = header
	a.footer = footer
	a.AddAndSwitch(NewServices(repos, a))

	tviewApp.SetInputCapture(a.handleKey)

	return a
}

func (a *Application) handleKey(event *tcell.EventKey) *tcell.EventKey {
	// the filter input handles its own keys, including Enter and Escape
	if a.filterInput != nil {
		return event
	}

	frontName, _ := a.pages.GetFrontPage()

	// dismiss overlays with Escape without disturbing the navigation stack
	if frontName == "picker" || frontName == "error" {
		if event.Key() == tcell.KeyEscape {
			a.pages.RemovePage(frontName)
			a.app.SetFocus(a.pages)
			return nil
		}
		return event
	}

	if event.Key() == tcell.KeyEscape {
		// clear an active filter before leaving the view
		if f, ok := a.components[len(a.components)-1].(Filterable); ok && f.GetFilter() != "" {
			f.SetFilter("")
			a.footer.Render()
			return nil
		}
		a.Close()
		return nil
	}

	// pass down Enter keypress to the component
	if event.Key() == tcell.KeyEnter {
		return event
	}

	actions := a.GetActiveKeyActions()
	for _, action := range actions {
		if event.Name() == action.Key.Name() {
			action.Action()
			return nil
		}
	}
	return event
}

func (a *Application) GetRegion() string {
	return a.region
}

func (a *Application) rebuild(profile, region string) {
	a.cancel()
	a.ctx, a.cancel = context.WithCancel(context.Background())

	cfg, err := loadConfig(profile, region)
	if err != nil {
		a.ShowError(err)
		return
	}

	a.profile = profile
	a.region = cfg.Region

	repos, stsRepo := buildRepos(cfg)
	a.header.UpdateRepos(stsRepo, repos.IAM)

	// Walk the current navigation stack backward to find the deepest component
	// that is a generic list view (i.e. its key exists in the registry). Detail
	// views for specific resources have a resource ID as their first label, so
	// they won't match. We stop at the first match and recreate from there.
	var resumeKey string
	for i := len(a.components) - 1; i >= 0; i-- {
		labels := a.components[i].GetLabels()
		if len(labels) == 0 {
			continue
		}
		key := a.components[i].GetService() + "." + labels[0]
		if _, ok := registry[key]; ok {
			resumeKey = key
			break
		}
	}

	a.clearPages()

	a.AddAndSwitch(NewServices(repos, a))
	if resumeKey != "" {
		a.AddAndSwitch(registry[resumeKey](repos, a))
	}
}

// clearPages removes all components from the navigation stack.
func (a *Application) clearPages() {
	for a.pages.GetPageCount() > 0 {
		name, _ := a.pages.GetFrontPage()
		if name == "" {
			break
		}
		a.pages.RemovePage(name)
	}
	// Once it has no pages left, tview.Pages focuses its own box without the
	// application knowing, so it never gets blurred and keeps claiming focus
	// over e.g. the filter input. Focusing it properly lets the next added
	// page take over the focus.
	a.app.SetFocus(a.pages)
	a.components = nil
}

func (a *Application) ShowError(err error) {
	modal := tview.NewModal().
		SetText(err.Error()).
		SetTextColor(tcell.ColorRed).
		SetBackgroundColor(tcell.ColorDefault).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(_ int, _ string) {
			a.pages.RemovePage("error")
			a.app.SetFocus(a.pages)
		})
	modal.Box.SetBackgroundColor(tcell.ColorDefault)
	a.pages.AddPage("error", modal, false, true)
	a.app.SetFocus(modal)
}

func (a *Application) refreshHandler() {
	_, primitive := a.pages.GetFrontPage()
	if err := primitive.(Component).Render(a.ctx); err != nil {
		a.ShowError(err)
	}
}

func (a *Application) GetActiveKeyActions() []KeyAction {
	// TODO check that front page exists
	_, primitive := a.pages.GetFrontPage()
	// TODO avoid type coercion
	localActions := primitive.(Component).GetKeyActions()
	globalActions := []KeyAction{
		{
			Key:         tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModCtrl),
			Description: "Refresh",
			Action:      a.refreshHandler,
		},
		{
			Key:         tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModCtrl),
			Description: "Switch Region",
			Action:      a.showRegionPicker,
		},
		{
			Key:         tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModCtrl),
			Description: "Switch Profile",
			Action:      a.showProfilePicker,
		},
	}
	if filterable, ok := primitive.(Filterable); ok {
		globalActions = append(globalActions, KeyAction{
			Key:         tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone),
			Description: "Filter",
			Action:      func() { a.showFilter(filterable) },
		})
	}
	if sortable, ok := primitive.(Sortable); ok {
		globalActions = append(globalActions, KeyAction{
			Key:         tcell.NewEventKey(tcell.KeyRune, 'S', tcell.ModNone),
			Description: "Sort",
			Action:      func() { a.showSortPicker(sortable) },
		})
	}
	return append(localActions, globalActions...)
}

// showFilter replaces the footer with an input that filters f as you type.
// Enter keeps the filter, Escape clears it.
func (a *Application) showFilter(f Filterable) {
	input := tview.NewInputField().
		SetLabel("/").
		SetText(f.GetFilter()).
		SetFieldBackgroundColor(tcell.ColorDefault).
		SetLabelColor(tcell.ColorYellow)
	input.SetChangedFunc(f.SetFilter)
	input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			f.SetFilter("")
		}
		a.layout.RemoveItem(input)
		a.layout.AddItem(a.footer, 1, 0, false)
		a.filterInput = nil
		a.app.SetFocus(a.pages)
		a.footer.Render()
	})
	a.layout.RemoveItem(a.footer)
	a.layout.AddItem(input, 1, 0, true)
	a.filterInput = input
	a.app.SetFocus(input)
}

func (a *Application) AddAndSwitch(v Component) {
	if err := v.Render(a.ctx); err != nil {
		a.ShowError(err)
		return
	}
	// create a unique name for the tview pages element
	// TODO this hardcodes the index as part of the name to avoid collisions when similar views are chained together
	name := fmt.Sprintf("%v | %v | %v ", a.pages.GetPageCount(), v.GetService(), strings.Join(v.GetLabels(), " > "))
	a.components = append(a.components, v)
	a.pages.AddAndSwitchToPage(name, v, true)
	a.header.Render(a.ctx) // this has to happen after we update the pages view
	a.footer.Render()
	a.pages.SetTitle(fmt.Sprintf(" %v ", v.GetService()))
}

func (a *Application) Close() {
	// don't close if we're at the root page
	if a.pages.GetPageCount() == 1 {
		return
	}
	a.components = a.components[:len(a.components)-1]

	oldName, _ := a.pages.GetFrontPage()
	a.pages.RemovePage(oldName)
	// this assumes pages are retrieved in reverse order that they were added
	newName, _ := a.pages.GetFrontPage()
	a.pages.SwitchToPage(newName)
	a.pages.SetTitle(fmt.Sprintf(" %v ", a.components[len(a.components)-1].GetService()))
	a.header.Render(a.ctx)
	a.footer.Render()
}

func (a Application) Run() error {
	return a.app.Run()
}
