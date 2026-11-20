<script lang="ts" module>
  // Colored file and folder icons from Material Icon Theme (MIT), see
  // src/assets/material-icons/README.md. UI chrome icons stay in Icon.svelte.
  import settings from "../assets/material-icons/settings.svg";
  import consoleIcon from "../assets/material-icons/console.svg";
  import image from "../assets/material-icons/image.svg";
  import video from "../assets/material-icons/video.svg";
  import audio from "../assets/material-icons/audio.svg";
  import palette from "../assets/material-icons/palette.svg";
  import documentIcon from "../assets/material-icons/document.svg";
  import folderBase from "../assets/material-icons/folder-base.svg";
  import folderScripts from "../assets/material-icons/folder-scripts.svg";
  import folderImages from "../assets/material-icons/folder-images.svg";
  import folderAudio from "../assets/material-icons/folder-audio.svg";

  export const fileIcons = {
    def: settings,
    code: consoleIcon,
    sprite: image,
    anim: video,
    sound: audio,
    palette,
    file: documentIcon,
    folder: folderBase,
    "folder-states": folderScripts,
    "folder-graphics": folderImages,
    "folder-sound": folderAudio,
  } as const;
  export type FileIconName = keyof typeof fileIcons;

  // fileIconFor picks the icon for a file of the character, by its DEF role
  // and name.
  export function fileIconFor(role: string, name: string): FileIconName {
    if (role === "sprite" || /\.sff$/i.test(name)) return "sprite";
    if (role === "anim" || /\.air$/i.test(name)) return "anim";
    if (role === "sound" || /\.snd$/i.test(name)) return "sound";
    if (role.startsWith("pal") || /\.act$/i.test(name)) return "palette";
    if (/\.def$/i.test(name)) return "def";
    if (/\.(cns|st|cmd|zss|const)$/i.test(name)) return "code";
    return "file";
  }
</script>

<script lang="ts">
  let { name, size = 16 }: { name: FileIconName; size?: number } = $props();
</script>

<img class="file-icon" src={fileIcons[name]} width={size} height={size} alt="" draggable="false" />

<style>
  .file-icon { flex: none; display: block; }
</style>
