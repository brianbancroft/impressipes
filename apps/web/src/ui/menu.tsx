import * as M from '@radix-ui/react-dropdown-menu';
import { cn } from './cn';

export const Menu = M.Root;
export const MenuTrigger = M.Trigger;
export const MenuSeparator = () => <M.Separator className="my-1 h-[1.5px] bg-line" />;
export const MenuLabel = ({ children }: { children: React.ReactNode }) => <M.Label className="px-3 pt-2 pb-1 eyebrow">{children}</M.Label>;

export function MenuContent({ className, align = 'end', ...props }: M.DropdownMenuContentProps) {
  return (
    <M.Portal>
      <M.Content
        align={align}
        sideOffset={8}
        className={cn('z-50 min-w-48 rounded-md border-[2.5px] border-outline bg-surface p-1.5 text-ink shadow-pop', className)}
        {...props}
      />
    </M.Portal>
  );
}

export function MenuItem({ className, danger, ...props }: M.DropdownMenuItemProps & { danger?: boolean }) {
  return (
    <M.Item
      className={cn(
        'flex min-h-11 cursor-pointer items-center gap-2.5 rounded-sm px-3 font-bold outline-none select-none data-highlighted:bg-surface-2 [&_svg]:size-4.5',
        danger && 'text-danger',
        className,
      )}
      {...props}
    />
  );
}

export const MenuRadioGroup = M.RadioGroup;
export function MenuRadioItem({ className, children, ...props }: M.DropdownMenuRadioItemProps) {
  return (
    <M.RadioItem
      className={cn('flex min-h-11 cursor-pointer items-center gap-2.5 rounded-sm px-3 font-bold outline-none select-none data-highlighted:bg-surface-2', className)}
      {...props}
    >
      <span className="grid size-5 place-items-center rounded-full border-2 border-outline">
        <M.ItemIndicator><span className="block size-2.5 rounded-full bg-accent-fill" /></M.ItemIndicator>
      </span>
      {children}
    </M.RadioItem>
  );
}
