import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';
import { forwardRef } from 'react';
import { cn } from './cn';

export const buttonVariants = cva(
  'inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap border-[2.5px] border-outline font-bold no-underline transition-[transform,box-shadow] select-none disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-5 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        primary: 'bg-accent text-on-accent shadow-pop hover:text-on-accent hover:-translate-y-px active:translate-x-0.5 active:translate-y-0.5 active:shadow-none',
        secondary: 'bg-surface text-ink shadow-pop hover:text-ink hover:-translate-y-px active:translate-x-0.5 active:translate-y-0.5 active:shadow-none',
        sticker: 'bg-butter text-sticker-ink shadow-pop-sm hover:text-sticker-ink hover:-translate-y-px active:translate-x-0.5 active:translate-y-0.5 active:shadow-none',
        flat: 'bg-surface text-ink hover:bg-surface-2 hover:text-ink',
        ghost: 'border-transparent bg-transparent text-ink hover:bg-surface-2 hover:text-ink',
        link: 'border-0 bg-transparent p-0 text-center whitespace-normal text-accent underline underline-offset-2 hover:text-accent-hover',
        dashed: 'border-dashed bg-transparent text-ink hover:bg-surface-2 hover:text-ink',
      },
      size: {
        sm: 'min-h-10 rounded-sm px-3.5 text-sm',
        md: 'min-h-12 rounded-md px-5 text-base',
        lg: 'min-h-14 rounded-md px-6 text-lg',
        icon: 'size-11.5 rounded-full p-0',
        'icon-sm': 'size-10 rounded-full p-0 [&_svg]:size-4.5',
      },
    },
    defaultVariants: { variant: 'secondary', size: 'md' },
  },
);

type Props = React.ButtonHTMLAttributes<HTMLButtonElement> & VariantProps<typeof buttonVariants> & { asChild?: boolean };

export const Button = forwardRef<HTMLButtonElement, Props>(({ className, variant, size, asChild, type, ...props }, ref) => {
  const Comp = asChild ? Slot : 'button';
  return <Comp ref={ref} type={asChild ? undefined : (type ?? 'button')} className={cn(buttonVariants({ variant, size }), className)} {...props} />;
});
Button.displayName = 'Button';
