package freepsutils

import (
	"github.com/hannesrauhe/freeps/base"
)

// OpMath is a freeps operator that provides math operations
type OpMath struct {
}

var _ base.FreepsOperator = &OpMath{}

type BinaryIntOperationArgs struct {
	Left  int
	Right int
}

type BinaryFloatOperationArgs struct {
	Left  float64
	Right float64
}

func (o *OpMath) AddInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	return base.MakeIntegerOutput(args.Left + args.Right)
}

func (o *OpMath) AddFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	return base.MakeFloatOutput(args.Left + args.Right)
}

func (o *OpMath) SubtractInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	return base.MakeIntegerOutput(args.Left - args.Right)
}

func (o *OpMath) SubtractFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	return base.MakeFloatOutput(args.Left - args.Right)
}

func (o *OpMath) MultiplyInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	return base.MakeIntegerOutput(args.Left * args.Right)
}

func (o *OpMath) MultiplyFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	return base.MakeFloatOutput(args.Left * args.Right)
}

func (o *OpMath) DivideInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	return base.MakeIntegerOutput(args.Left / args.Right)
}

func (o *OpMath) DivideFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	return base.MakeFloatOutput(args.Left / args.Right)
}

func (o *OpMath) ModInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	return base.MakeIntegerOutput(args.Left % args.Right)
}

func (o *OpMath) ModFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	return base.MakeFloatOutput(float64(int(args.Left) % int(args.Right)))
}

func (o *OpMath) NegateInt(ctx *base.Context, input *base.OperatorIO, args int) *base.OperatorIO {
	return base.MakeIntegerOutput(-args)
}

func (o *OpMath) NegateFloat(ctx *base.Context, input *base.OperatorIO, args float64) *base.OperatorIO {
	return base.MakeFloatOutput(-args)
}

func (o *OpMath) AbsInt(ctx *base.Context, input *base.OperatorIO, args int) *base.OperatorIO {
	if args < 0 {
		return base.MakeIntegerOutput(-args)
	}
	return base.MakeIntegerOutput(args)
}

func (o *OpMath) AbsFloat(ctx *base.Context, input *base.OperatorIO, args float64) *base.OperatorIO {
	if args < 0 {
		return base.MakeFloatOutput(-args)
	}
	return base.MakeFloatOutput(args)
}

func (o *OpMath) MaxInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	if args.Left > args.Right {
		return base.MakeIntegerOutput(args.Left)
	}
	return base.MakeIntegerOutput(args.Right)
}

func (o *OpMath) MaxFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	if args.Left > args.Right {
		return base.MakeFloatOutput(args.Left)
	}
	return base.MakeFloatOutput(args.Right)
}

func (o *OpMath) MinInt(ctx *base.Context, input *base.OperatorIO, args BinaryIntOperationArgs) *base.OperatorIO {
	if args.Left < args.Right {
		return base.MakeIntegerOutput(args.Left)
	}
	return base.MakeIntegerOutput(args.Right)
}

func (o *OpMath) MinFloat(ctx *base.Context, input *base.OperatorIO, args BinaryFloatOperationArgs) *base.OperatorIO {
	if args.Left < args.Right {
		return base.MakeFloatOutput(args.Left)
	}
	return base.MakeFloatOutput(args.Right)
}
