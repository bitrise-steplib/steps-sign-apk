android_home = ENV['ANDROID_HOME']
if android_home.nil? || android_home == ''
  puts 'Failed to get ANDROID_HOME env'
  exit 1
end

zipalign_files = Dir[File.join(android_home, 'build-tools', '/**/zipalign')]
unless zipalign_files
  puts 'Failed to find zipalign tool'
  exit 1
end

latest_build_tool_version = ''
latest_zipalign_path = ''
zipalign_files.each do |zipalign_file|
  path_splits = zipalign_file.to_s.split('/')
  build_tool_version = path_splits[path_splits.count - 2]

  latest_build_tool_version = build_tool_version if latest_build_tool_version == ''
  if Gem::Version.new(build_tool_version) >= Gem::Version.new(latest_build_tool_version)
    latest_build_tool_version = build_tool_version
    latest_zipalign_path = zipalign_file.to_s
  end
end

if latest_zipalign_path == ''
  puts 'Failed to find latest zipalign tool'
  exit 1
end

puts latest_zipalign_path
exit 0
