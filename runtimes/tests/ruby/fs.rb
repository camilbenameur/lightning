["/etc/lightning", "/workspace/../lightning", "/workspace/lightning", "/tmp/lightning"].each do |path|
  File.write(path, "")
  puts "#{path}: ok"
rescue SystemCallError
  puts "#{path}: error"
end
