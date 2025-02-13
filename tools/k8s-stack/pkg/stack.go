package ledger_stack

import (
	"fmt"
	"github.com/formancehq/go-libs/v2/pointer"
	pulumi_ledger "github.com/formancehq/ledger/deployments/pulumi/pkg"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumix"
	"strings"
	"time"
)

type StackComponent struct {
	pulumi.ResourceState
	Ledger   *pulumi_ledger.Component
	Postgres *PostgresComponent
}

type StackLedgerArgs struct {
	Version     pulumix.Input[string]
	GracePeriod pulumix.Input[string]
	Upgrade     pulumix.Input[pulumi_ledger.UpgradeMode]
	Otel        *pulumi_ledger.OtelArgs
}

type StackComponentArgs struct {
	Debug     pulumix.Input[bool]
	Namespace pulumix.Input[string]
	Ledger    *StackLedgerArgs
}

func NewStack(ctx *pulumi.Context, name string, args *StackComponentArgs, opts ...pulumi.ResourceOption) (*StackComponent, error) {
	cmp := &StackComponent{}
	err := ctx.RegisterComponentResource("Formance:Ledger:Testing", name, cmp, opts...)
	if err != nil {
		return nil, err
	}

	resourceOptions := []pulumi.ResourceOption{
		pulumi.Parent(cmp),
	}

	var namespaceName pulumix.Input[string]
	if args.Namespace != nil {
		namespaceName = pulumix.Apply(args.Namespace, func(namespace string) string {
			if namespace == "" {
				return fmt.Sprintf("%s-%s", ctx.Project(), strings.Replace(ctx.Stack(), ".", "-", -1))
			}
			return namespace
		})
	} else {
		namespaceName = pulumi.Sprintf("%s-%s", ctx.Project(), strings.Replace(ctx.Stack(), ".", "-", -1))
	}

	namespace, err := corev1.NewNamespace(ctx, "namespace", &corev1.NamespaceArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name: namespaceName.ToOutput(ctx.Context()).Untyped().(pulumi.StringOutput),
		},
	}, pulumi.Parent(cmp))
	if err != nil {
		return nil, fmt.Errorf("creating namespace: %w", err)
	}

	resourceOptions = append(resourceOptions, pulumi.DependsOn([]pulumi.Resource{namespace}))

	cmp.Postgres, err = NewPostgresComponent(ctx, "postgres", &PostgresComponentArgs{
		Namespace: namespaceName,
	}, resourceOptions...)
	if err != nil {
		return nil, fmt.Errorf("creating Postgres component: %w", err)
	}

	cmp.Ledger, err = pulumi_ledger.NewComponent(ctx, "ledger", &pulumi_ledger.ComponentArgs{
		Timeout:         pulumi.Int(30),
		Debug:           args.Debug,
		Tag:             args.Ledger.Version,
		ImagePullPolicy: pulumi.String("Always"),
		Postgres: pulumi_ledger.PostgresArgs{
			URI: pulumi.Sprintf(
				"postgres://%s:%s@%s:%d/postgres?sslmode=disable",
				cmp.Postgres.Username,
				cmp.Postgres.Password,
				cmp.Postgres.Host,
				cmp.Postgres.Port,
			),
			MaxIdleConns:    pulumix.Val(pointer.For(100)),
			MaxOpenConns:    pulumix.Val(pointer.For(100)),
			ConnMaxIdleTime: pulumix.Val(pointer.For(time.Minute)),
		},
		ExperimentalFeatures: pulumi.Bool(true),
		Namespace: pulumix.Apply(namespace.Metadata.Name().ToOutput(ctx.Context()).Untyped().(pulumi.StringPtrOutput), func(ns *string) string {
			return *ns
		}),
		GracePeriod: args.Ledger.GracePeriod,
		Upgrade:     args.Ledger.Upgrade,
		Otel:        args.Ledger.Otel,
	},
		append(resourceOptions, pulumi.DependsOn([]pulumi.Resource{
			cmp.Postgres,
		}))...,
	)

	if err := ctx.RegisterResourceOutputs(cmp, pulumi.Map{}); err != nil {
		return nil, fmt.Errorf("registering outputs: %w", err)
	}

	return cmp, nil
}
