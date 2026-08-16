-- ====================================================================
-- soneph : AppleScript Folder Action (macOS -> Musique)
-- ====================================================================
-- ⚠️ Déprécié : préfère scripts/watch_and_import.sh (sans doublons).
-- Ce script surveille le dossier local synchronisé par Syncthing.
-- Dès qu'un fichier MP3 y est déposé par le VPS:
-- 1. Il l'ajoute dans l'app Musique.
-- 2. Il lit le fichier .lrc, nettoie les horodatages [00:xx.xx]
--    et injecte les paroles propres dans Musique.
-- 3. Avec la bibliothèque iCloud, le morceau et ses paroles sont
--    instantanément disponibles sur Mac et iPhone !
-- ====================================================================

on adding folder items to this_folder after receiving added_items
	repeat with i from 1 to number of items in added_items
		try
			set this_item to item i of added_items
			set item_info to info for this_item
			
			if not (folder of item_info) then
				set file_extension to name extension of item_info
				
				if file_extension is in {"mp3", "m4a", "flac"} then
					set posix_path to POSIX path of this_item
					set lrc_path to text 1 thru -5 of posix_path & ".lrc"
					set lrc_content to ""
					
					-- Lire le fichier .lrc s'il existe
					try
						set lrc_file to open for access POSIX file lrc_path
						set lrc_content to read lrc_file as «class utf8»
						close access lrc_file
					on error
						try
							close access POSIX file lrc_path
						end try
					end try
					
					-- Nettoyer les balises [00:xx.xx] pour Musique
					if lrc_content is not "" then
						set clean_lyrics to ""
						set AppleScript's text item delimiters to linefeed
						set lrc_lines to text items of lrc_content
						repeat with a_line in lrc_lines
							if a_line contains "]" then
								set AppleScript's text item delimiters to "]"
								set line_parts to text items of a_line
								if (count of line_parts) > 1 then
									set clean_text to text item 2 of a_line
									if clean_text is not "" then
										set clean_lyrics to clean_lyrics & clean_text & linefeed
									end if
								end if
							end if
						end repeat
						set AppleScript's text item delimiters to ""
						set lrc_content to clean_lyrics
					end if
					
					-- Importer dans Musique et définir les paroles
					tell application "Music"
						set new_tracks to (add this_item)
						if (count of new_tracks) > 0 and lrc_content is not "" then
							repeat with a_track in new_tracks
								set lyrics of a_track to lrc_content
							end repeat
						end if
					end tell
				end if
			end if
		on error errText
			-- Ignorer les verrouillages temporaires pendant le transfert Syncthing
		end try
	end repeat
end adding folder items to
