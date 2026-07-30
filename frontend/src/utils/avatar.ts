const avatarColors = ["#0f766e", "#2563eb", "#7c3aed", "#c2410c", "#be185d", "#4f46e5"] as const;

export function getAvatarInitial(displayName: string) {
  const value = Array.from(displayName.trim()).find((character) => /\S/u.test(character));
  return value ? /[a-z]/iu.test(value) ? value.toUpperCase() : value : "?";
}

export function getStableAvatarColor(displayName: string) {
  let hash = 0;
  for (const character of displayName) hash = (hash * 31 + character.codePointAt(0)!) >>> 0;
  return avatarColors[hash % avatarColors.length];
}
