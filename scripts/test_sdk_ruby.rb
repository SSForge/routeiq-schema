# Verifies the generated Ruby SDK:
# - All three telemetry proto files load without error.
# - Key message classes are defined.
# - RetrievalEvent has cache_hit field (the worked-example field).

$LOAD_PATH.unshift("out/ruby/lib")

require "google/protobuf"

# Load generated files in dependency order.
load "out/ruby/lib/routeiq/proto/routeiq/v1/telemetry/entities_pb.rb"
load "out/ruby/lib/routeiq/proto/routeiq/v1/telemetry/events_pb.rb"

expected_classes = %w[
  Routeiq::V1::Telemetry::AgentEvent
  Routeiq::V1::Telemetry::TaskEvent
  Routeiq::V1::Telemetry::ToolCallEvent
  Routeiq::V1::Telemetry::StepEvent
  Routeiq::V1::Telemetry::DecisionEvent
  Routeiq::V1::Telemetry::RetrievalEvent
  Routeiq::V1::Telemetry::MemoryEvent
  Routeiq::V1::Telemetry::HandoffEvent
  Routeiq::V1::Telemetry::PolicyEvent
  Routeiq::V1::Telemetry::InterventionEvent
  Routeiq::V1::Telemetry::RecoveryEvent
  Routeiq::V1::Telemetry::StateSnapshotEvent
]

errors = []

expected_classes.each do |klass_name|
  begin
    klass = Object.const_get(klass_name)
    # Instantiate to confirm the class is a proper proto message descriptor.
    klass.new
  rescue NameError, NoMethodError => e
    errors << "#{klass_name}: #{e.message}"
  end
end

# Verify cache_hit field on RetrievalEvent.
r = Routeiq::V1::Telemetry::RetrievalEvent.new(cache_hit: true)
unless r.cache_hit == true
  errors << "RetrievalEvent.cache_hit field not working"
end

if errors.any?
  errors.each { |e| puts "ERROR: #{e}" }
  exit 1
end

puts "Ruby SDK: OK — #{expected_classes.size} message classes and cache_hit field verified"
