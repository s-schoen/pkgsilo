import IconDeviceDesktop from "@tabler/icons-react/icons/IconDeviceDesktop";
import IconMoon from "@tabler/icons-react/icons/IconMoon";
import IconSun from "@tabler/icons-react/icons/IconSun";

import { Button } from "@/components/ui/button";
import { Menu, MenuItem, MenuTrigger } from "@/components/ui/menu";
import { Tooltip, TooltipTrigger } from "@/components/ui/tooltip";
import { useColorScheme } from "@/features/appearance/hooks/use-color-scheme.ts";
import { colorSchemePreferences } from "@/features/appearance/utils/color-scheme.ts";
import { m } from "@/paraglide/messages.js";

import type { ColorSchemePreference } from "@/features/appearance/utils/color-scheme.ts";

// The name and the icon of each preference.
const options = {
  system: { label: m.colorSchemeSystem, icon: IconDeviceDesktop },
  light: { label: m.colorSchemeLight, icon: IconSun },
  dark: { label: m.colorSchemeDark, icon: IconMoon },
} satisfies Record<ColorSchemePreference, unknown>;

function isPreference(key: unknown): key is ColorSchemePreference {
  return colorSchemePreferences.includes(key as ColorSchemePreference);
}

// The button of the header that picks the color scheme, System, Light or Dark, from a menu; it
// shows the icon of the picked one, and this browser keeps the choice.
export function ColorSchemeMenu() {
  const [preference, setPreference] = useColorScheme();

  return (
    <MenuTrigger>
      <TooltipTrigger>
        <Button variant="ghost" icon={options[preference].icon} aria-label={m.colorScheme()} />
        <Tooltip placement="bottom">{m.colorScheme()}</Tooltip>
      </TooltipTrigger>
      <Menu
        aria-label={m.colorScheme()}
        placement="bottom end"
        selectionMode="single"
        disallowEmptySelection
        selectedKeys={[preference]}
        onSelectionChange={(keys) => {
          const [key] = keys === "all" ? [] : keys;
          if (isPreference(key)) {
            setPreference(key);
          }
        }}
      >
        {colorSchemePreferences.map((id) => (
          <MenuItem key={id} id={id} icon={options[id].icon}>
            {options[id].label()}
          </MenuItem>
        ))}
      </Menu>
    </MenuTrigger>
  );
}
