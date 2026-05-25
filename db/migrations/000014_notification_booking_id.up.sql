ALTER TABLE notifications ADD COLUMN booking_id UUID REFERENCES bookings(id);
CREATE INDEX idx_notifications_booking_id ON notifications(booking_id);
